package main

import (
 "context"
 "encoding/json"
 "errors"
 "os"
 "path/filepath"
 "strings"
 "testing"
 "time"
)

type fakeJobPanel struct {
 response *claimResponse
 body []byte
 claimErr error
 ackErr error
 resultErr error
 acks int
 renewals int
 renewErr error
 results []jobResultRequest
 claims int
 reconcileCalls int
 reconcileStatus string
 reconcileErr error
}
func (f *fakeJobPanel) claim(context.Context) (*claimResponse, []byte, error) { f.claims++; return f.response, f.body, f.claimErr }
func (f *fakeJobPanel) acknowledge(context.Context, string, string) error { f.acks++; return f.ackErr }
func (f *fakeJobPanel) renew(context.Context, string, string) (time.Time, error) { f.renewals++; return time.Now().Add(time.Minute), f.renewErr }
func (f *fakeJobPanel) reportResult(_ context.Context, _ string, result jobResultRequest) error { f.results = append(f.results, result); return f.resultErr }
func (f *fakeJobPanel) reconcile(context.Context, string, string, string, string, map[string]any) (string, error) { f.reconcileCalls++; return f.reconcileStatus, f.reconcileErr }

type fakeJobExecutor struct { calls int; err error }
func (f *fakeJobExecutor) Execute(context.Context, signedNodeJob) (map[string]any, error) {
 f.calls++
 return map[string]any{"executed": true}, f.err
}
func testPollerJob(t *testing.T, action string) (*claimResponse, []byte) {
 t.Helper()
 now := time.Now().UTC()
 body := signedJobBody(now, action, `{}`)
 return &claimResponse{Signature: "sha256=" + signJob(body), LeaseToken: testLeaseToken, LeaseExpiresAt: now.Add(time.Minute)}, body
}
func newTestPoller(panel *fakeJobPanel, executor *fakeJobExecutor, dir string) *nodeJobPoller {
 return &nodeJobPoller{panel: panel, executor: executor, nodeID: "node-a", signingKey: jobTestSecret, journalPath: filepath.Join(dir, "replay.jsonl"), interval: time.Second, leaseRenewInterval: 30 * time.Second}
}
func TestPollerRequiresExecutorBeforePolling(t *testing.T) {
 p := newTestPoller(&fakeJobPanel{}, &fakeJobExecutor{}, t.TempDir())
 p.executor = nil
 if err := p.validate(); err == nil { t.Fatal("poller must not claim work without an executor") }
}
func TestPollerEmptyQueueDoesNothing(t *testing.T) {
 panel, executor := &fakeJobPanel{}, &fakeJobExecutor{}
 p := newTestPoller(panel, executor, t.TempDir())
 if err := p.pollOnce(context.Background()); err != nil { t.Fatal(err) }
 if panel.acks != 0 || len(panel.results) != 0 || executor.calls != 0 { t.Fatal("empty queue must not change state") }
}
func TestPollerDoesNotExecuteDuplicateDelivery(t *testing.T) {
 response, body := testPollerJob(t, "START")
 panel, executor := &fakeJobPanel{response: response, body: body}, &fakeJobExecutor{}
 p := newTestPoller(panel, executor, t.TempDir())
 if err := p.pollOnce(context.Background()); err != nil { t.Fatal(err) }
 if executor.calls != 1 || panel.acks != 1 || len(panel.results) != 1 { t.Fatalf("first delivery: calls=%d acks=%d results=%d", executor.calls, panel.acks, len(panel.results)) }
 panel.acks, panel.results = 0, nil
 if err := p.pollOnce(context.Background()); err != nil { t.Fatal(err) }
 if executor.calls != 1 || panel.acks != 1 || len(panel.results) != 0 { t.Fatalf("duplicate delivery executed: calls=%d acks=%d results=%d", executor.calls, panel.acks, len(panel.results)) }
}
func TestPollerReportsExecutionFailure(t *testing.T) {
 response, body := testPollerJob(t, "STOP")
 panel, executor := &fakeJobPanel{response: response, body: body}, &fakeJobExecutor{err: errors.New("runtime unavailable")}
 p := newTestPoller(panel, executor, t.TempDir())
 if err := p.pollOnce(context.Background()); err == nil { t.Fatal("expected execution failure") }
 if len(panel.results) != 1 || panel.results[0].Status != "FAILED" || panel.results[0].ErrorCode != "EXECUTION_FAILED" { t.Fatalf("unexpected failure report: %+v", panel.results) }
}

func TestBoundedErrorTrimsAndLimitsMessage(t *testing.T) {
 if got := boundedError("  runtime unavailable  ", 20); got != "runtime unavailable" { t.Fatalf("unexpected trimmed error: %q", got) }
 if got := boundedError("123456789", 5); got != "12345" { t.Fatalf("unexpected bounded error: %q", got) }
 if got := boundedError("   ", 5); got != "" { t.Fatalf("expected empty trimmed error, got %q", got) }
}

func TestPollerRequiresSafeLeaseRenewalInterval(t *testing.T) {
 p := newTestPoller(&fakeJobPanel{}, &fakeJobExecutor{}, t.TempDir())
 p.leaseRenewInterval = 31 * time.Second
 if err := p.validate(); err == nil { t.Fatal("expected excessive lease renewal interval to be rejected") }
}

func TestPollerDoesNotExecuteWhenAcknowledgementFails(t *testing.T) {
 response, body := testPollerJob(t, "START")
 panel, executor := &fakeJobPanel{response: response, body: body, ackErr: errors.New("ack timeout")}, &fakeJobExecutor{}
 p := newTestPoller(panel, executor, t.TempDir())
 if err := p.pollOnce(context.Background()); err == nil { t.Fatal("expected acknowledgement failure") }
 if executor.calls != 0 || len(panel.results) != 0 {
  t.Fatalf("job executed after failed ACK: calls=%d results=%d", executor.calls, len(panel.results))
 }
}

func TestPollerReturnsErrorWhenResultSubmissionFails(t *testing.T) {
 response, body := testPollerJob(t, "START")
 panel, executor := &fakeJobPanel{response: response, body: body, resultErr: errors.New("result endpoint unavailable")}, &fakeJobExecutor{}
 p := newTestPoller(panel, executor, t.TempDir())
 if err := p.pollOnce(context.Background()); err == nil { t.Fatal("expected result submission failure") }
 if executor.calls != 1 || len(panel.results) != 1 {
  t.Fatalf("expected one execution and one attempted result: calls=%d results=%d", executor.calls, len(panel.results))
 }
}

func readOperationJournalStages(t *testing.T, path string) []operationStage {
 t.Helper()
 data, err := os.ReadFile(path)
 if err != nil { t.Fatal(err) }
 lines := strings.Split(strings.TrimSpace(string(data)), "\n")
 stages := make([]operationStage, 0, len(lines))
 for _, line := range lines {
  var record operationJournalRecord
  if err := json.Unmarshal([]byte(line), &record); err != nil { t.Fatal(err) }
  stages = append(stages, record.Stage)
 }
 return stages
}

func TestPollerPersistsLifecycleBeforeAndAfterExecution(t *testing.T) {
 response, body := testPollerJob(t, "START")
 panel, executor := &fakeJobPanel{response: response, body: body}, &fakeJobExecutor{}
 p := newTestPoller(panel, executor, t.TempDir())
 if err := p.pollOnce(context.Background()); err != nil { t.Fatal(err) }
 stages := readOperationJournalStages(t, p.journalPath+".operations.jsonl")
 want := []operationStage{stageAccepted, stageAckConfirmed, stageExecutionStarted, stageResultPending, stageResultConfirmed}
 if len(stages) != len(want) { t.Fatalf("stages=%v, want %v", stages, want) }
 for i := range want { if stages[i] != want[i] { t.Fatalf("stages=%v, want %v", stages, want) } }
}

func TestPollerLeavesDurablePendingResultWhenSubmissionFails(t *testing.T) {
 response, body := testPollerJob(t, "STOP")
 panel, executor := &fakeJobPanel{response: response, body: body, resultErr: errors.New("panel unavailable")}, &fakeJobExecutor{}
 p := newTestPoller(panel, executor, t.TempDir())
 if err := p.pollOnce(context.Background()); err == nil { t.Fatal("expected result submission failure") }
 stages := readOperationJournalStages(t, p.journalPath+".operations.jsonl")
 if stages[len(stages)-1] != stageResultPending { t.Fatalf("last stage=%s, want RESULT_PENDING", stages[len(stages)-1]) }
}

func TestPollerPersistsAcceptanceButNotExecutionWhenAckFails(t *testing.T) {
 response, body := testPollerJob(t, "START")
 panel, executor := &fakeJobPanel{response: response, body: body, ackErr: errors.New("ack timeout")}, &fakeJobExecutor{}
 p := newTestPoller(panel, executor, t.TempDir())
 if err := p.pollOnce(context.Background()); err == nil { t.Fatal("expected ACK failure") }
 stages := readOperationJournalStages(t, p.journalPath+".operations.jsonl")
 if len(stages) != 1 || stages[0] != stageAccepted || executor.calls != 0 {
  t.Fatalf("stages=%v executor calls=%d", stages, executor.calls)
 }
}

func TestPollerBlocksNewClaimsWhileAnOperationIsUnresolved(t *testing.T) {
 panel, executor := &fakeJobPanel{}, &fakeJobExecutor{}
 p := newTestPoller(panel, executor, t.TempDir())
 unresolved := operationTestRecord(stageAccepted)
 if err := advanceOperationJournal(p.journalPath+".operations.jsonl", unresolved); err != nil { t.Fatal(err) }
 unresolved.Stage = stageAckConfirmed
 if err := advanceOperationJournal(p.journalPath+".operations.jsonl", unresolved); err != nil { t.Fatal(err) }
 unresolved.Stage = stageExecutionStarted
 if err := advanceOperationJournal(p.journalPath+".operations.jsonl", unresolved); err != nil { t.Fatal(err) }
 if err := p.pollOnce(context.Background()); err == nil || !strings.Contains(err.Error(), "reconciliation is required") {
  t.Fatalf("expected unresolved-operation block, got %v", err)
 }
 if panel.claims != 0 || executor.calls != 0 {
  t.Fatalf("unsafe progress after unresolved operation: claims=%d executions=%d", panel.claims, executor.calls)
 }
}

func TestPollerReconcilesPendingResultBeforeClaiming(t *testing.T) {
 panel := &fakeJobPanel{reconcileStatus: "RESULT_ACCEPTED"}
 executor := &fakeJobExecutor{}
 p := newTestPoller(panel, executor, t.TempDir())
 record := operationTestRecord(stageAccepted)
 path := p.journalPath + ".operations.jsonl"
 if err := advanceOperationJournal(path, record); err != nil { t.Fatal(err) }
 record.Stage = stageAckConfirmed
 if err := advanceOperationJournal(path, record); err != nil { t.Fatal(err) }
 record.Stage = stageExecutionStarted
 if err := advanceOperationJournal(path, record); err != nil { t.Fatal(err) }
 record.Stage = stageResultPending
 record.Result = json.RawMessage(`{"status":"SUCCEEDED","result":{"ok":true}}`)
 record.ResultHash = replayHash(string(record.Result))
 if err := advanceOperationJournal(path, record); err != nil { t.Fatal(err) }
 if err := p.pollOnce(context.Background()); err != nil { t.Fatal(err) }
 if panel.reconcileCalls != 1 || panel.claims != 1 || executor.calls != 0 {
  t.Fatalf("unexpected recovery sequence: reconciles=%d claims=%d executions=%d", panel.reconcileCalls, panel.claims, executor.calls)
 }
 latest, err := latestOperationJournalRecords(path)
 if err != nil { t.Fatal(err) }
 if latest[record.JobID].Stage != stageResultConfirmed { t.Fatalf("stage=%s, want RESULT_CONFIRMED", latest[record.JobID].Stage) }
}

func TestPollerKeepsPendingResultBlockedWhenPanelIsNotReady(t *testing.T) {
 panel := &fakeJobPanel{reconcileStatus: "IN_PROGRESS"}
 p := newTestPoller(panel, &fakeJobExecutor{}, t.TempDir())
 record := operationTestRecord(stageAccepted)
 path := p.journalPath + ".operations.jsonl"
 if err := advanceOperationJournal(path, record); err != nil { t.Fatal(err) }
 record.Stage = stageAckConfirmed
 if err := advanceOperationJournal(path, record); err != nil { t.Fatal(err) }
 record.Stage = stageExecutionStarted
 if err := advanceOperationJournal(path, record); err != nil { t.Fatal(err) }
 record.Stage = stageResultPending
 record.Result = json.RawMessage(`{"status":"SUCCEEDED"}`)
 record.ResultHash = replayHash(string(record.Result))
 if err := advanceOperationJournal(path, record); err != nil { t.Fatal(err) }
 if err := p.pollOnce(context.Background()); err == nil || !strings.Contains(err.Error(), "IN_PROGRESS") {
  t.Fatalf("expected pending reconciliation block, got %v", err)
 }
 if panel.claims != 0 { t.Fatalf("claimed new work while result pending: %d", panel.claims) }
}

func TestPollerJournalsLegacyDuplicateBeforeReAck(t *testing.T) {
 response, body := testPollerJob(t, "START")
 panel, executor := &fakeJobPanel{response: response, body: body}, &fakeJobExecutor{}
 p := newTestPoller(panel, executor, t.TempDir())
 _, duplicate, err := acceptSignedNodeJob(body, response.Signature, p.signingKey, p.nodeID, p.journalPath, time.Now().UTC())
 if err != nil || duplicate { t.Fatalf("seed replay journal duplicate=%v err=%v", duplicate, err) }
 if err := p.pollOnce(context.Background()); err != nil { t.Fatal(err) }
 if executor.calls != 0 || panel.acks != 1 { t.Fatalf("duplicate executed: calls=%d acks=%d", executor.calls, panel.acks) }
 stages := readOperationJournalStages(t, p.journalPath+".operations.jsonl")
 if len(stages) != 1 || stages[0] != stageAccepted { t.Fatalf("legacy duplicate stages=%v", stages) }
 if err := p.pollOnce(context.Background()); err == nil || !strings.Contains(err.Error(), "operator reconciliation is required") {
  t.Fatalf("expected unresolved duplicate to block future claims, got %v", err)
 }
}
