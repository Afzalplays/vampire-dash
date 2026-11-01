package main

import (
 "context"
 "crypto/sha256"
 "encoding/hex"
 "encoding/json"
 "errors"
 "fmt"
 "log"
 "os"
 "strings"
 "time"
)

type jobPanelAPI interface {
 claim(context.Context) (*claimResponse, []byte, error)
 acknowledge(context.Context, string, string) error
 renew(context.Context, string, string) (time.Time, error)
 reportResult(context.Context, string, jobResultRequest) error
}

// nodeJobExecutor must map allowlisted actions to fixed runtime operations.
// Implementations must never execute user-controlled shell commands.
type nodeJobExecutor interface {
 Execute(context.Context, signedNodeJob) (map[string]any, error)
}

type nodeJobPoller struct {
 panel jobPanelAPI
 executor nodeJobExecutor
 nodeID string
 signingKey string
 journalPath string
 interval time.Duration
 leaseRenewInterval time.Duration
}

func (p *nodeJobPoller) validate() error {
 if p.panel == nil || p.executor == nil { return errors.New("job poller requires a panel client and an execution adapter") }
 if p.leaseRenewInterval < time.Second || p.leaseRenewInterval > 30*time.Second { return errors.New("lease renewal interval must be between 1 and 30 seconds") }
 if p.nodeID == "" || len(p.signingKey) < 32 { return errors.New("job poller requires a node ID and a 256-bit signing key") }
 if p.journalPath == "" { return errors.New("job poller requires a persistent replay journal path") }
 if p.interval < time.Second || p.interval > time.Minute { return errors.New("job poll interval must be between 1 second and 1 minute") }
 return nil
}

// run polls sequentially and refuses to claim work without a concrete executor.
func (p *nodeJobPoller) run(ctx context.Context) error {
 if err := p.validate(); err != nil { return err }
 ticker := time.NewTicker(p.interval)
 defer ticker.Stop()
 for {
  if err := p.pollOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
   // Never log job payloads, lease tokens, or signatures.
   log.Printf("node job poll failed: %v", err)
  }
  select {
  case <-ctx.Done(): return ctx.Err()
  case <-ticker.C:
  }
 }
}

// pollOnce persists replay protection before ACK. An identical redelivery only
// retries ACK and is never executed again.
func (p *nodeJobPoller) pollOnce(ctx context.Context) error {
 if err := p.ensureNoUnresolvedOperations(); err != nil { return err }
 response, signedBody, err := p.panel.claim(ctx)
 if err != nil || response == nil { return err }
 job, duplicate, err := acceptSignedNodeJob(signedBody, response.Signature, p.signingKey, p.nodeID, p.journalPath, time.Now().UTC())
 if err != nil { return fmt.Errorf("reject signed node job: %w", err) }
 identity, err := stableNodeJobIdentity(job)
 if err != nil { return fmt.Errorf("derive operation identity: %w", err) }
 identitySum := sha256.Sum256(identity)
 identityHash := hex.EncodeToString(identitySum[:])
 operationPath := p.journalPath + ".operations.jsonl"
 record := operationJournalRecord{JobID: job.ID, NodeID: job.NodeID, Nonce: job.Nonce, IdentityHash: identityHash, Stage: stageAccepted}
 // A duplicate may be an old operation whose execution/result was interrupted.
 // Re-ACK only; never execute it again without the future reconciliation protocol.
 if duplicate { return p.panel.acknowledge(ctx, job.ID, response.LeaseToken) }
 if err := advanceOperationJournal(operationPath, record); err != nil { return fmt.Errorf("persist accepted operation: %w", err) }
 if err := p.panel.acknowledge(ctx, job.ID, response.LeaseToken); err != nil { return fmt.Errorf("acknowledge accepted job: %w", err) }
 record.Stage = stageAckConfirmed
 if err := advanceOperationJournal(operationPath, record); err != nil { return fmt.Errorf("persist acknowledged operation: %w", err) }
 record.Stage = stageExecutionStarted
 if err := advanceOperationJournal(operationPath, record); err != nil { return fmt.Errorf("persist execution intent: %w", err) }

 result, executeErr := p.executeWithLeaseRenewal(ctx, job, response.LeaseToken)
 report := jobResultRequest{LeaseToken: response.LeaseToken, Status: "SUCCEEDED", Result: result}
 if executeErr != nil {
  report.Status = "FAILED"
  report.ErrorCode = "EXECUTION_FAILED"
  report.ErrorMessage = boundedError(executeErr.Error(), 500)
 }
 // Persist the exact terminal payload without the ephemeral lease credential.
 terminal := map[string]any{"status": report.Status}
 if report.Result != nil { terminal["result"] = report.Result }
 if report.ErrorCode != "" { terminal["errorCode"] = report.ErrorCode }
 if report.ErrorMessage != "" { terminal["errorMessage"] = report.ErrorMessage }
 resultBytes, err := json.Marshal(terminal)
 if err != nil { return fmt.Errorf("encode terminal operation result: %w", err) }
 resultSum := sha256.Sum256(resultBytes)
 record.Stage = stageResultPending
 record.Result = resultBytes
 record.ResultHash = hex.EncodeToString(resultSum[:])
 if err := advanceOperationJournal(operationPath, record); err != nil { return fmt.Errorf("persist pending operation result: %w", err) }
 if err := p.panel.reportResult(ctx, job.ID, report); err != nil { return fmt.Errorf("report job result: %w", err) }
 record.Stage = stageResultConfirmed
 if err := advanceOperationJournal(operationPath, record); err != nil { return fmt.Errorf("persist confirmed operation result: %w", err) }
 if executeErr != nil { return fmt.Errorf("job %s execution failed: %w", job.ID, executeErr) }
 return nil
}


func (p *nodeJobPoller) ensureNoUnresolvedOperations() error {
 latest, err := latestOperationJournalRecords(p.journalPath + ".operations.jsonl")
 if errors.Is(err, os.ErrNotExist) { return nil }
 if err != nil { return fmt.Errorf("inspect operation recovery journal: %w", err) }
 for jobID, record := range latest {
  if record.Stage != stageResultConfirmed {
   return fmt.Errorf("operation %s remains %s; reconciliation is required before claiming more work", jobID, record.Stage)
  }
 }
 return nil
}

func boundedError(value string, max int) string {
 value = strings.TrimSpace(value)
 if len(value) <= max { return value }
 return value[:max]
}


// executeWithLeaseRenewal keeps acknowledged work leased while the executor
// runs. A renewal failure cancels the execution context and is reported as a
// failed operation; executors must honor context cancellation.
func (p *nodeJobPoller) executeWithLeaseRenewal(ctx context.Context, job signedNodeJob, leaseToken string) (map[string]any, error) {
 execCtx, cancel := context.WithCancel(ctx)
 defer cancel()
 renewalErr := make(chan error, 1)
 stopped := make(chan struct{})
 go func() {
  defer close(stopped)
  ticker := time.NewTicker(p.leaseRenewInterval)
  defer ticker.Stop()
  for {
   select {
   case <-execCtx.Done():
    return
   case <-ticker.C:
    if _, err := p.panel.renew(execCtx, job.ID, leaseToken); err != nil {
     select { case renewalErr <- fmt.Errorf("renew job lease: %w", err): default: }
     cancel()
     return
    }
   }
  }
 }()
 result, executeErr := p.executor.Execute(execCtx, job)
 cancel()
 <-stopped
 select {
 case err := <-renewalErr:
  if executeErr != nil { return nil, fmt.Errorf("%v; execution stopped: %w", err, executeErr) }
  return nil, err
 default:
 }
 return result, executeErr
}
