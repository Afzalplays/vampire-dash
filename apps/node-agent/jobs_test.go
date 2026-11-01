package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"
)

const jobTestSecret = "01234567890123456789012345678901"

func signedJobBody(now time.Time, action, payload string) []byte {
	return []byte(fmt.Sprintf(`{"id":"job_identifier_1234","nodeId":"node-a","nonce":"0123456789abcdef0123456789abcdef","issuedAt":%q,"expiresAt":%q,"action":%q,"payload":%s}`,
		now.UTC().Format(time.RFC3339), now.Add(time.Minute).UTC().Format(time.RFC3339), action, payload))
}

func signJob(body []byte) string {
	mac := hmac.New(sha256.New, []byte(jobTestSecret))
	_, _ = mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

func TestValidateSignedPowerJob(t *testing.T) {
	now := time.Now().UTC()
	body := signedJobBody(now, "RESTART", `{}`)
	job, err := validateSignedNodeJob(body, signJob(body), jobTestSecret, "node-a", now)
	if err != nil { t.Fatal(err) }
	if job.Action != "RESTART" || job.NodeID != "node-a" { t.Fatalf("unexpected job: %+v", job) }
}

func TestValidateSignedPropertiesJob(t *testing.T) {
	now := time.Now().UTC()
	body := signedJobBody(now, "UPDATE_PROPERTIES", `{"motd":"Welcome","maxPlayers":40,"pvp":true}`)
	if _, err := validateSignedNodeJob(body, signJob(body), jobTestSecret, "node-a", now); err != nil { t.Fatal(err) }
}

func TestRejectTamperedOrWrongNodeJob(t *testing.T) {
	now := time.Now().UTC()
	body := signedJobBody(now, "STOP", `{}`)
	if _, err := validateSignedNodeJob(body, signJob(body), jobTestSecret, "node-b", now); err == nil { t.Fatal("expected wrong-node rejection") }
	body = append(body, ' ')
	if _, err := validateSignedNodeJob(body, signJob(body)+"00", jobTestSecret, "node-a", now); err == nil { t.Fatal("expected malformed signature rejection") }
}

func TestRejectUnknownActionsAndPayloadFields(t *testing.T) {
	now := time.Now().UTC()
	for _, test := range []struct{ action, payload string }{
		{"RUN_COMMAND", `{"command":"rm -rf /"}`},
		{"START", `{"shell":"anything"}`},
		{"UPDATE_PROPERTIES", `{"serverPath":"/etc"}`},
		{"UPDATE_PROPERTIES", `{"difficulty":"impossible"}`},
	} {
		body := signedJobBody(now, test.action, test.payload)
		if _, err := validateSignedNodeJob(body, signJob(body), jobTestSecret, "node-a", now); err == nil {
			t.Fatalf("expected rejection for %s %s", test.action, test.payload)
		}
	}
}

func TestRejectExpiredAndOverlongJobs(t *testing.T) {
	now := time.Now().UTC()
	expired := signedJobBody(now.Add(-3*time.Minute), "START", `{}`)
	if _, err := validateSignedNodeJob(expired, signJob(expired), jobTestSecret, "node-a", now); err == nil { t.Fatal("expected expired job rejection") }
	body := make([]byte, maxJobBodyBytes+1)
	if _, err := validateSignedNodeJob(body, signJob(body), jobTestSecret, "node-a", now); err == nil { t.Fatal("expected oversized body rejection") }
}
