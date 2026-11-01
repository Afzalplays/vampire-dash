package main

import (
	"testing"
	"time"
)

func TestAcceptSignedJobMarksIdenticalRetryAsDuplicate(t *testing.T) {
	now := time.Now().UTC()
	body := signedJobBody(now, "RESTART", "{}")
	path := t.TempDir() + "/state/replay.jsonl"
	_, duplicate, err := acceptSignedNodeJob(body, signJob(body), jobTestSecret, "node-a", path, now)
	if err != nil || duplicate { t.Fatalf("first accept duplicate=%v err=%v", duplicate, err) }
	_, duplicate, err = acceptSignedNodeJob(body, signJob(body), jobTestSecret, "node-a", path, now)
	if err != nil || !duplicate { t.Fatalf("retry duplicate=%v err=%v", duplicate, err) }
	changed := signedJobBody(now, "STOP", "{}")
	if _, _, err = acceptSignedNodeJob(changed, signJob(changed), jobTestSecret, "node-a", path, now); err == nil {
		t.Fatal("expected conflicting job ID reuse rejection")
	}
}

func TestAcceptSignedJobTreatsReissuedLeaseAsSameOperation(t *testing.T) {
	now := time.Now().UTC()
	first := signedJobBody(now, "RESTART", "{}")
	path := t.TempDir() + "/state/replay.jsonl"
	if _, duplicate, err := acceptSignedNodeJob(first, signJob(first), jobTestSecret, "node-a", path, now); err != nil || duplicate {
		t.Fatalf("first accept duplicate=%v err=%v", duplicate, err)
	}
	reissued := signedJobBody(now.Add(10*time.Second), "RESTART", "{ }")
	if _, duplicate, err := acceptSignedNodeJob(reissued, signJob(reissued), jobTestSecret, "node-a", path, now.Add(10*time.Second)); err != nil || !duplicate {
		t.Fatalf("reissued lease duplicate=%v err=%v", duplicate, err)
	}
}

func TestAcceptSignedJobRejectsChangedPayloadOnReissue(t *testing.T) {
	now := time.Now().UTC()
	path := t.TempDir() + "/state/replay.jsonl"
	first := signedJobBody(now, "UPDATE_PROPERTIES", `{"motd":"original"}`)
	if _, _, err := acceptSignedNodeJob(first, signJob(first), jobTestSecret, "node-a", path, now); err != nil { t.Fatal(err) }
	changed := signedJobBody(now.Add(10*time.Second), "UPDATE_PROPERTIES", `{"motd":"changed"}`)
	if _, _, err := acceptSignedNodeJob(changed, signJob(changed), jobTestSecret, "node-a", path, now.Add(10*time.Second)); err == nil {
		t.Fatal("expected changed operation content to be rejected")
	}
}

func TestInvalidSignatureDoesNotConsumeReplayKey(t *testing.T) {
	now := time.Now().UTC()
	body := signedJobBody(now, "START", "{}")
	path := t.TempDir() + "/state/replay.jsonl"
	if _, _, err := acceptSignedNodeJob(body, "00", jobTestSecret, "node-a", path, now); err == nil { t.Fatal("expected invalid signature") }
	if _, duplicate, err := acceptSignedNodeJob(body, signJob(body), jobTestSecret, "node-a", path, now); err != nil || duplicate {
		t.Fatalf("valid retry duplicate=%v err=%v", duplicate, err)
	}
}
