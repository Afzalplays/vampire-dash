package main

import (
	"testing"
	"time"
)

func TestAcceptSignedJobConsumesReplayKeyOnce(t *testing.T) {
	now := time.Now().UTC()
	body := signedJobBody(now, "RESTART", `{}`)
	path := t.TempDir() + "/state/replay.jsonl"
	if _, err := acceptSignedNodeJob(body, signJob(body), jobTestSecret, "node-a", path, now); err != nil {
		t.Fatal(err)
	}
	if _, err := acceptSignedNodeJob(body, signJob(body), jobTestSecret, "node-a", path, now); err == nil {
		t.Fatal("expected replay rejection")
	}
}

func TestInvalidSignatureDoesNotConsumeReplayKey(t *testing.T) {
	now := time.Now().UTC()
	body := signedJobBody(now, "START", `{}`)
	path := t.TempDir() + "/state/replay.jsonl"
	if _, err := acceptSignedNodeJob(body, "00", jobTestSecret, "node-a", path, now); err == nil {
		t.Fatal("expected invalid signature rejection")
	}
	if _, err := acceptSignedNodeJob(body, signJob(body), jobTestSecret, "node-a", path, now); err != nil {
		t.Fatalf("valid retry should still be accepted: %v", err)
	}
}
