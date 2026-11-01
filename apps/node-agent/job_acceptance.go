package main

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// acceptSignedNodeJob validates the exact signed bytes and durably records them.
// duplicate=true means the same envelope was already accepted: callers may
// retry its acknowledgement, but MUST NOT execute the job again.
func acceptSignedNodeJob(body []byte, signature, secret, nodeID, journalPath string, now time.Time) (signedNodeJob, bool, error) {
	job, err := validateSignedNodeJob(body, signature, secret, nodeID, now)
	if err != nil { return signedNodeJob{}, false, err }
	hash := sha256.Sum256(body)
	duplicate, err := consumeJobReplayKey(journalPath, job.ID, job.Nonce, hex.EncodeToString(hash[:]))
	if err != nil { return signedNodeJob{}, false, err }
	return job, duplicate, nil
}
