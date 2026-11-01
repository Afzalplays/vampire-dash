package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"
)

// acceptSignedNodeJob authenticates the exact signed bytes, then reserves a
// stable identity derived from immutable job fields. Lease timestamps may
// change when the panel reissues a job, but its ID/nonce/action/payload may not.
func acceptSignedNodeJob(body []byte, signature, secret, nodeID, journalPath string, now time.Time) (signedNodeJob, bool, error) {
	job, err := validateSignedNodeJob(body, signature, secret, nodeID, now)
	if err != nil {
		return signedNodeJob{}, false, err
	}
	identity, err := stableNodeJobIdentity(job)
	if err != nil {
		return signedNodeJob{}, false, err
	}
	hash := sha256.Sum256(identity)
	duplicate, err := consumeJobReplayKey(journalPath, job.ID, job.Nonce, hex.EncodeToString(hash[:]))
	if err != nil {
		return signedNodeJob{}, false, err
	}
	return job, duplicate, nil
}

// stableNodeJobIdentity excludes issuedAt/expiresAt because those are lease
// metadata, not operation identity. JSON payload objects are canonicalized so
// insignificant whitespace and key ordering do not create a new operation.
func stableNodeJobIdentity(job signedNodeJob) ([]byte, error) {
	var payload any
	decoder := json.NewDecoder(bytes.NewReader(job.Payload))
	decoder.UseNumber()
	if err := decoder.Decode(&payload); err != nil {
		return nil, err
	}
	canonicalPayload, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return json.Marshal(struct {
		ID string `json:"id"`
		NodeID string `json:"nodeId"`
		Nonce string `json:"nonce"`
		Action string `json:"action"`
		Payload json.RawMessage `json:"payload"`
	}{job.ID, job.NodeID, job.Nonce, job.Action, canonicalPayload})
}
