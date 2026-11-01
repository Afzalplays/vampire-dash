# Node-agent job protocol (design gate)

## Current implementation status

The agent currently exposes read-only health and capacity telemetry. The signed job envelope validator in `jobs.go` and durable replay journal in
`replay_store.go` are tested protocol foundations. The agent still has no job
HTTP polling loop, container runtime, or command execution. The panel must continue to report
server operations as `NOT_DISPATCHED`.

The API has a durable `NodeAgentJob` outbox record per operation, bound to
one node, with a unique nonce, bounded retry count, lease token hash and lease
expiry fields. It exposes node-authenticated claim, acknowledgement, renewal and
result endpoints. Claims use an atomic `FOR UPDATE SKIP LOCKED` selection and
sign the exact serialized envelope with a per-node HMAC key. Lease tokens are
random, stored only as SHA-256 hashes, and required for acknowledgement,
renewal and result reporting. The API validates queued payloads against strict
action schemas before signing. The `worker:agent-jobs` recovery process retries
expired pre-ack leases with bounded backoff and fails expired acknowledged
leases without replaying potentially side-effecting work.

The Go agent has strict signed-envelope validation and an append-only,
fsync-backed replay journal protected by a file lock. The journal stores the SHA-256 digest of the exact signed body. A byte-identical
redelivery is recognized as an already accepted duplicate, allowing a client
to retry acknowledgement after a crash without treating the job as new. Reuse
of a job ID or nonce with different content fails closed. A caller receiving a
duplicate must never execute it; it may only retry the acknowledgement. This
journal is not yet connected to a job polling loop.
The existing panel-to-node mTLS client currently fetches read-only metrics;
there is no job HTTP client or execution runtime. Actual job delivery and
execution remain disabled.

## Envelope

The panel-to-agent request body is one JSON object with:

- `id`: opaque job identifier (16-128 URL-safe characters)
- `nodeId`: exact registered node identity
- `nonce`: cryptographically random 16-64 byte hex value
- `issuedAt`, `expiresAt`: RFC3339 timestamps, lifetime no more than 2 minutes
- `action`: allowlisted typed action
- `payload`: action-specific JSON object

The exact raw body is authenticated with HMAC-SHA-256 in an
`X-Vampire-Signature: sha256=<hex>` header. Transport must additionally use
mutually authenticated TLS. The HMAC key is unique per node, at least 256 bits,
and never shared between nodes. The agent rejects unknown envelope and payload
fields, oversized bodies, stale timestamps, wrong node IDs and unknown actions.

## Initial action allowlist

- `START`, `STOP`, `RESTART`: empty payload only.
- `UPDATE_PROPERTIES`: strict allowlist of MOTD, difficulty, gamemode, max
  players, PvP, online mode, flight, view distance, simulation distance and
  spawn protection with bounded values.

Raw shell strings, arbitrary URLs, Docker arguments, host paths, environment
variable injection and user-defined executable commands are forbidden.

## Required before enabling dispatch

1. Integrate the durable agent replay journal with a job HTTP client. Design
   idempotent acknowledgement recovery for a crash after journal fsync but
   before acknowledgement; never execute a duplicate side effect.
2. Implement an agent polling client that sends the exact signed-body bytes,
   validates the panel's mTLS identity, acknowledges only after durable
   acceptance, renews active leases and reports bounded results. API-side
   claim/ack/renew/result and lease recovery are implemented.
3. Add node credential rotation and revocation; retain mandatory mTLS
   certificate verification and per-node HMAC keys.
4. Add a typed runtime adapter that maps actions to fixed container APIs; never
   invoke a shell or accept user-controlled container identifiers/options.
5. Enforce server-to-node ownership, per-server resource boundaries, rate
   limits and audit events before dispatch.
6. Test duplicate delivery, clock skew, expired leases, node restarts, partial
   failures and compromised/incorrect node credentials.
7. Keep operations marked undispatched until a worker actually claims them.

No production dispatch should be enabled until every gate above is implemented
and reviewed.
