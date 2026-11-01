package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func operationTestRecord(stage operationStage) operationJournalRecord {
	return operationJournalRecord{
		JobID: "job_identifier_1234", NodeID: "node-test", Nonce: replayTestNonce,
		IdentityHash: replayHash("immutable-operation"), Stage: stage,
	}
}

func TestOperationJournalPersistsMonotonicLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "operations.jsonl")
	stages := []operationStage{stageAccepted, stageAckConfirmed, stageExecutionStarted}
	for _, stage := range stages {
		if err := advanceOperationJournal(path, operationTestRecord(stage)); err != nil {
			t.Fatalf("advance %s: %v", stage, err)
		}
	}
	result := json.RawMessage(`{"status":"SUCCEEDED"}`)
	sum := sha256.Sum256(result)
	pending := operationTestRecord(stageResultPending)
	pending.Result = result
	pending.ResultHash = hex.EncodeToString(sum[:])
	if err := advanceOperationJournal(path, pending); err != nil {
		t.Fatalf("persist pending result: %v", err)
	}
	confirmed := pending
	confirmed.Stage = stageResultConfirmed
	if err := advanceOperationJournal(path, confirmed); err != nil {
		t.Fatalf("confirm result: %v", err)
	}
	// Repeating the exact final transition is idempotent.
	if err := advanceOperationJournal(path, confirmed); err != nil {
		t.Fatalf("idempotent confirmation: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permissions=%o, want 600", info.Mode().Perm())
	}
}

func TestOperationJournalRejectsSkippedOrBackwardTransitions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.jsonl")
	if err := advanceOperationJournal(path, operationTestRecord(stageAccepted)); err != nil {
		t.Fatal(err)
	}
	if err := advanceOperationJournal(path, operationTestRecord(stageExecutionStarted)); err == nil {
		t.Fatal("expected skipped transition rejection")
	}
	if err := advanceOperationJournal(path, operationTestRecord(stageAckConfirmed)); err != nil {
		t.Fatal(err)
	}
	if err := advanceOperationJournal(path, operationTestRecord(stageAccepted)); err == nil {
		t.Fatal("expected backward transition rejection")
	}
}

func TestOperationJournalRejectsIdentityAndResultConflicts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.jsonl")
	if err := advanceOperationJournal(path, operationTestRecord(stageAccepted)); err != nil {
		t.Fatal(err)
	}
	conflict := operationTestRecord(stageAccepted)
	conflict.IdentityHash = replayHash("different-operation")
	if err := advanceOperationJournal(path, conflict); err == nil {
		t.Fatal("expected operation identity conflict")
	}
	if err := advanceOperationJournal(path, operationTestRecord(stageAckConfirmed)); err != nil {
		t.Fatal(err)
	}
	if err := advanceOperationJournal(path, operationTestRecord(stageExecutionStarted)); err != nil {
		t.Fatal(err)
	}
	result := json.RawMessage(`{"status":"SUCCEEDED"}`)
	sum := sha256.Sum256(result)
	pending := operationTestRecord(stageResultPending)
	pending.Result, pending.ResultHash = result, hex.EncodeToString(sum[:])
	if err := advanceOperationJournal(path, pending); err != nil {
		t.Fatal(err)
	}
	other := pending
	other.Result = json.RawMessage(`{"status":"FAILED"}`)
	otherSum := sha256.Sum256(other.Result)
	other.ResultHash = hex.EncodeToString(otherSum[:])
	if err := advanceOperationJournal(path, other); err == nil {
		t.Fatal("expected contradictory result rejection")
	}
}

func TestOperationJournalFailsClosedOnCorruptionAndInvalidResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "operations.jsonl")
	if err := os.WriteFile(path, []byte("{broken\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := advanceOperationJournal(path, operationTestRecord(stageAccepted)); err == nil || !strings.Contains(err.Error(), "corrupt") {
		t.Fatalf("expected corruption error, got %v", err)
	}

	badPath := filepath.Join(t.TempDir(), "bad-result.jsonl")
	bad := operationTestRecord(stageResultPending)
	bad.Result = json.RawMessage(`{"status":"SUCCEEDED"}`)
	bad.ResultHash = replayHash("not-the-result")
	if err := advanceOperationJournal(badPath, bad); err == nil {
		t.Fatal("expected result hash validation failure")
	}
}

func TestOperationJournalRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte(""), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "operations.jsonl")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := advanceOperationJournal(link, operationTestRecord(stageAccepted)); err == nil {
		t.Fatal("expected symlink rejection")
	}
}

func TestOperationJournalRecoveryReaderReturnsLatestPendingState(t *testing.T) {
 path := filepath.Join(t.TempDir(), "operations.jsonl")
 accepted := operationTestRecord(stageAccepted)
 if err := advanceOperationJournal(path, accepted); err != nil { t.Fatal(err) }
 acked := accepted
 acked.Stage = stageAckConfirmed
 if err := advanceOperationJournal(path, acked); err != nil { t.Fatal(err) }
 started := acked
 started.Stage = stageExecutionStarted
 if err := advanceOperationJournal(path, started); err != nil { t.Fatal(err) }
 result := json.RawMessage(`{"status":"SUCCEEDED"}`)
 sum := sha256.Sum256(result)
 pending := started
 pending.Stage, pending.Result, pending.ResultHash = stageResultPending, result, hex.EncodeToString(sum[:])
 if err := advanceOperationJournal(path, pending); err != nil { t.Fatal(err) }
 latest, err := latestOperationJournalRecords(path)
 if err != nil { t.Fatal(err) }
 got, ok := latest[accepted.JobID]
 if !ok || got.Stage != stageResultPending || got.ResultHash != pending.ResultHash {
  t.Fatalf("latest record=%+v, found=%v", got, ok)
 }
}

func TestOperationJournalRecoveryReaderRejectsIncompleteHistory(t *testing.T) {
 path := filepath.Join(t.TempDir(), "operations.jsonl")
 skipped := operationTestRecord(stageExecutionStarted)
 encoded, err := json.Marshal(skipped)
 if err != nil { t.Fatal(err) }
 if err := os.WriteFile(path, append(encoded, '\n'), 0600); err != nil { t.Fatal(err) }
 if _, err := latestOperationJournalRecords(path); err == nil || !strings.Contains(err.Error(), "incomplete") {
  t.Fatalf("expected incomplete history error, got %v", err)
 }
}

func TestOperationJournalRecoveryReaderHardensPermissions(t *testing.T) {
 path := filepath.Join(t.TempDir(), "operations.jsonl")
 record := operationTestRecord(stageAccepted)
 encoded, err := json.Marshal(record)
 if err != nil { t.Fatal(err) }
 if err := os.WriteFile(path, append(encoded, '\n'), 0644); err != nil { t.Fatal(err) }
 if _, err := latestOperationJournalRecords(path); err != nil { t.Fatal(err) }
 info, err := os.Stat(path)
 if err != nil { t.Fatal(err) }
 if info.Mode().Perm() != 0600 { t.Fatalf("permissions=%o, want 600", info.Mode().Perm()) }
}
