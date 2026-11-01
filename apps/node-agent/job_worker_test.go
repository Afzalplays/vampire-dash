package main

import (
 "context"
 "errors"
 "path/filepath"
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
 results []jobResultRequest
}
func (f *fakeJobPanel) claim(context.Context) (*claimResponse, []byte, error) { return f.response, f.body, f.claimErr }
func (f *fakeJobPanel) acknowledge(context.Context, string, string) error { f.acks++; return f.ackErr }
func (f *fakeJobPanel) reportResult(_ context.Context, _ string, result jobResultRequest) error { f.results = append(f.results, result); return f.resultErr }

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
 return &nodeJobPoller{panel: panel, executor: executor, nodeID: "node-a", signingKey: jobTestSecret, journalPath: filepath.Join(dir, "replay.jsonl"), interval: time.Second}
}
func TestPollerRequiresExecutorBeforePolling(t *testing.T) {
 p := newTestPoller(&fakeJobPanel{}, nil, t.TempDir())
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
