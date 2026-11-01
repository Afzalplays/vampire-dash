# Node agent job protocol

The panel provides node-scoped claim, acknowledgement, lease renewal and result endpoints under `/api/agent/jobs`.

- Claim: `POST /claim` returns an allowlisted typed envelope, exact UTF-8 signed body, HMAC-SHA256 signature and one-time lease token. A node can claim only its own pending jobs.
- Acknowledgement: `POST /:jobId/ack` proves possession of the lease token and transitions the operation to RUNNING. The lease remains attached to the job. Repeating ACK with the same lease token while the lease is live returns the same ACKNOWLEDGED response; a different token or expired lease is rejected.
- Renewal: `POST /:jobId/renew` extends a live LEASED or ACKNOWLEDGED lease by 90 seconds. The agent must renew before expiry.
- Result: `POST /:jobId/result` accepts SUCCEEDED or FAILED from an acknowledged job and updates the operation atomically. Repeating the exact same result request is idempotent; a different result for an already terminal job is rejected.
- Lease tokens are random 256-bit values; only SHA-256 hashes are stored. Comparisons use constant-time equality. Result payloads are capped at 8 KiB.
- Expired unacknowledged leases can be retried with bounded backoff. An acknowledged job whose lease expires is failed closed and is not replayed, because the agent may already have performed side effects.
- The node agent still has no execution/dispatch implementation in this stage. These APIs only manage job lifecycle metadata. Do not enable customer nodes until the agent's HTTPS+mTLS enrollment and execution sandbox have been reviewed and tested.
