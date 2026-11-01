package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

const replayTestJob = "job_identifier_1234"
const replayTestNonce = "0123456789abcdef0123456789abcdef"

func TestReplayJournalPersistsAcrossCalls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "replay.jsonl")
	if err := consumeJobReplayKey(path, replayTestJob, replayTestNonce); err != nil { t.Fatal(err) }
	if err := consumeJobReplayKey(path, replayTestJob, "abcdef0123456789abcdef0123456789"); err == nil { t.Fatal("expected duplicate job ID rejection") }
	if err := consumeJobReplayKey(path, "job_identifier_5678", replayTestNonce); err == nil { t.Fatal("expected duplicate nonce rejection") }
	if err := consumeJobReplayKey(path, "job_identifier_5678", "abcdef0123456789abcdef0123456789"); err != nil { t.Fatal(err) }
	info, err := os.Stat(path)
	if err != nil { t.Fatal(err) }
	if info.Mode().Perm() != 0600 { t.Fatalf("journal permissions = %o, want 600", info.Mode().Perm()) }
}

func TestReplayJournalRejectsInvalidKeysAndPaths(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay.jsonl")
	for _, tc := range []struct{ path, id, nonce string }{
		{path, "bad", replayTestNonce},
		{path, replayTestJob, "bad"},
		{"relative.jsonl", replayTestJob, replayTestNonce},
	} {
		if err := consumeJobReplayKey(tc.path, tc.id, tc.nonce); err == nil { t.Fatalf("expected rejection for %+v", tc) }
	}
}

func TestReplayJournalFailsClosedOnCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay.jsonl")
	if err := os.WriteFile(path, []byte("{broken\n"), 0600); err != nil { t.Fatal(err) }
	err := consumeJobReplayKey(path, replayTestJob, replayTestNonce)
	if err == nil || !strings.Contains(err.Error(), "corrupt") { t.Fatalf("expected corrupt journal error, got %v", err) }
}

func TestReplayJournalConcurrentDuplicateIsConsumedOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "replay.jsonl")
	var wg sync.WaitGroup
	results := make(chan error, 12)
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); results <- consumeJobReplayKey(path, replayTestJob, replayTestNonce) }()
	}
	wg.Wait()
	close(results)
	successes := 0
	for err := range results { if err == nil { successes++ } }
	if successes != 1 { t.Fatalf("successful consumes = %d, want 1", successes) }
}
