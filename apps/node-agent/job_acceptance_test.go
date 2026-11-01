package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"
	"strings"
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

func TestAcceptSignedJobRejectsChangedServerIdentity(t *testing.T) {
	now := time.Now().UTC()
	path := t.TempDir() + "/state/replay.jsonl"
	first := signedJobBody(now, "START", "{}")
	if _, duplicate, err := acceptSignedNodeJob(first, signJob(first), jobTestSecret, "node-a", path, now); err != nil || duplicate {
		t.Fatalf("first accept duplicate=%v err=%v", duplicate, err)
	}
	changed := []byte(strings.Replace(string(first), "server-test", "server-other", 1))
	if _, _, err := acceptSignedNodeJob(changed, signJob(changed), jobTestSecret, "node-a", path, now); err == nil {
		t.Fatal("expected changed server identity to be rejected")
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

func TestOperationAndResultHashesMatchPanelCanonicalJSON(t *testing.T) {
 job := signedNodeJob{
  ID: "job_identifier_1234", NodeID: "node-test", ServerID: "server-test",
  Nonce: "0123456789abcdef0123456789abcdef",
  Action: "UPDATE_PROPERTIES",
  Payload: []byte(`{"motd":"hello<&","maxPlayers":10}`),
 }
 identity, err := stableNodeJobIdentity(job)
 if err != nil { t.Fatal(err) }
 identityHash := sha256.Sum256(identity)
 if got := hex.EncodeToString(identityHash[:]); got != "94dc928d8d00f9f4df74195d30d1ca5bf74bd75fd3d87d8fd01f764eb1207eae" {
  t.Fatalf("identity hash=%s", got)
 }
 terminal := map[string]any{"status":"SUCCEEDED", "result":map[string]any{"motd":"hello<&"}}
 encoded, err := json.Marshal(terminal)
 if err != nil { t.Fatal(err) }
 resultHash := sha256.Sum256(encoded)
 if got := hex.EncodeToString(resultHash[:]); got != "da776a42514be63318a00037a51914df73355730d11a54bbb9dbb55a5e2a352e" {
  t.Fatalf("result hash=%s", got)
 }
}
