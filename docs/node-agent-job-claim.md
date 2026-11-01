# Node agent job claim protocol

The API exposes `POST /api/agent/jobs/claim` for registered node agents.

- Authentication uses the node-specific bearer token generated during node registration. The API looks up its keyed digest; it never logs or returns the credential.
- A node must be explicitly enabled and report `ONLINE`. New registrations remain disabled until secure enrollment is completed.
- Claims use a PostgreSQL row lock with `SKIP LOCKED`, scoped to the authenticated node. A job moves from `PENDING` to `LEASED`, its attempt counter increments, and only a SHA-256 hash of a random 256-bit lease token is persisted.
- The response contains the typed job envelope, the exact serialized `signedBody`, its HMAC-SHA256 signature, the one-time lease token, and lease expiry. The node must verify the signature over the exact UTF-8 bytes of `signedBody` and durably consume the job ID/nonce before accepting it.
- No execution endpoint or acknowledgement endpoint is enabled by this change. The claim API does not execute commands or expose host paths.
- Deploy the API behind HTTPS. Do not enable nodes until inbound agent authentication and mTLS enrollment have been fully configured and tested.
