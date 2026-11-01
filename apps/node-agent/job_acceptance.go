package main

import "time"

// acceptSignedNodeJob validates an authenticated envelope before durably
// consuming its replay key. Callers must not acknowledge or execute a job
// unless this function returns successfully.
func acceptSignedNodeJob(body []byte, signature, secret, nodeID, journalPath string, now time.Time) (signedNodeJob, error) {
	job, err := validateSignedNodeJob(body, signature, secret, nodeID, now)
	if err != nil {
		return signedNodeJob{}, err
	}
	if err := consumeJobReplayKey(journalPath, job.ID, job.Nonce); err != nil {
		return signedNodeJob{}, err
	}
	return job, nil
}
