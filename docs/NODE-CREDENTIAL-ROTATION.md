# Node agent credential rotation

Node bearer credentials can be rotated by an authenticated administrator:

`POST /api/admin/nodes/:nodeId/rotate-credentials`

The endpoint is rate-limited to five requests per hour per client.

## Safety behavior

- Requires a valid administrator session and `NODE_CREDENTIALS_KEY`.
- Generates a fresh 256-bit random base64url bearer token.
- Stores only an HMAC digest for authentication and AES-256-GCM ciphertext for outbound agent authentication.
- Revokes the previous token in the same database transaction that stores the replacement.
- Disables the node and resets its health state; it cannot claim jobs until an administrator re-enables it after reconfiguration and a successful health check.
- Refuses rotation while the node has any `ACKNOWLEDGED` job. Such a job may already have side effects and must be reconciled before credential replacement.
- Writes an audit event without recording either the old or new token.
- Returns the new token once. It is not retrievable from the node listing endpoint.

## Operator procedure

1. Resolve or wait for all acknowledged jobs on the node.
2. Call the rotation endpoint using an administrator session.
3. Copy the returned `agentToken` directly into the node agent's protected configuration.
4. Restart the agent and verify that its health check succeeds.
5. Re-enable the node in the admin panel.

If the response is lost after rotation, the token cannot be recovered. Rotate again after confirming there are no acknowledged jobs, then configure the newly returned token.

Do not paste bearer tokens into tickets, chat, shell history, or logs. Use a secrets manager or a root-readable configuration file.
