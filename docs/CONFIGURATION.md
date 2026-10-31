# Configuration

Copy apps/api/.env.example to apps/api/.env. Required values include PostgreSQL DATABASE_URL and a unique SESSION_SECRET of at least 32 characters. Set COOKIE_SECURE=true behind HTTPS in production. WEB_ORIGIN must be the exact trusted web origin.

## Node credential encryption

Before registering compute nodes, configure NODE_CREDENTIALS_KEY with 32 cryptographically random bytes encoded as Base64:

```bash
openssl rand -base64 32
```

Keep this key separate from SESSION_SECRET, restrict access to the API service account, and store it in your secrets manager or protected environment file. Losing or changing this key makes stored node credentials undecryptable; back it up securely. Do not commit the key or place it in a NEXT_PUBLIC_ variable. Node registration returns the raw bearer token once, stores an AES-256-GCM encrypted copy for future panel-to-agent requests, and stores a keyed digest for credential identity. Existing node rows with no encrypted credential must be re-enrolled before the panel can call them.

FamGateway credentials, when the integration is completed and reviewed, belong only in the API process environment as FAMGATEWAY_API_KEY. Never use a NEXT_PUBLIC_ prefix for secrets. Never commit environment files.

## Panel-to-node mTLS client

Configure these API-only paths to the trusted node-agent server CA, panel client certificate, and matching private key. The API verifies the node certificate and hostname using normal TLS verification; there is no insecure bypass. Keep the private key readable only by the API service account. Production startup requires all three values. Requests have a bounded timeout and response size. The admin node check stores the last successful capacity sample and marks failed checks offline; it does not enable provisioning or reserve resources.

## Automated node health polling

Run the dedicated API worker with `npm run worker:nodes` from `apps/api`. Set `NODE_HEALTH_INTERVAL_MS` to a value between 15000 and 600000 (default 60000). Keep one supervised worker process running alongside the API. PostgreSQL advisory locking prevents concurrent polling cycles if more than one worker is accidentally started. Nodes without encrypted agent credentials are skipped. The worker records telemetry, marks failed nodes offline, and writes audit events only when a node changes between online and offline, avoiding per-poll audit noise.

The worker holds a database transaction while it checks nodes so the advisory lock remains valid; configure node timeouts and node count so a cycle completes comfortably inside the 120-second transaction limit. Host available RAM and filesystem capacity are telemetry only, not allocatable capacity or a reservation guarantee.
