# Node-agent job protocol (implementation status and design gate)

## Current implementation status

The panel API now has a durable `NodeAgentJob` outbox record per operation, bound to one node, with a unique nonce, bounded retry count, lease-token hash and lease-expiry fields. Node-authenticated API endpoints implement claim, acknowledgement, lease renewal and result reporting. Claims use an atomic `FOR UPDATE SKIP LOCKED` selection and sign the exact serialized envelope with the per-node HMAC key. Lease tokens are random, stored only as SHA-256 hashes, and required for acknowledgement, renewal and result reporting. Queued payloads are validated against strict action schemas before signing.

The `worker:agent-jobs` recovery process retries expired pre-acknowledgement leases with bounded backoff. Expired acknowledged leases fail closed rather than replaying potentially side-effecting work.

The Go agent has strict signed-envelope validation and an append-only, fsync-backed replay journal protected by a file lock. The journal stores the SHA-256 digest of the exact signed body. A byte-identical redelivery is recognized as an already accepted duplicate; reuse of a job ID or nonce with different content fails closed. A duplicate must never execute again and may only be used to recover acknowledgement.

The Go panel HTTPS client now supports bounded claim, acknowledgement, renewal and result requests. It rejects non-HTTPS panel URLs, blocks redirects to prevent bearer-token disclosure, bounds response sizes and supports an optional private panel CA while retaining system trust roots. Its methods preserve and validate the exact signed-body bytes. The client and replay journal are not yet connected to a running polling/execution loop.

The agent currently exposes read-only health and capacity telemetry. It has no Docker runtime adapter or server-operation execution. Operations must remain undispatched until the crash-safe polling loop, typed runtime adapter, authorization checks and integration tests are complete.

## Envelope

The panel-to-agent job envelope contains:

- `id`: opaque job identifier (16-128 URL-safe characters)
- `nodeId`: exact registered node identity
- `nonce`: cryptographically random 16-64 byte hex value
- `issuedAt`, `expiresAt`: RFC3339 timestamps, lifetime no more than 2 minutes
- `action`: allowlisted typed action
- `payload`: action-specific JSON object

The exact raw body is authenticated with HMAC-SHA-256 as `sha256=<hex>`. Transport additionally requires mutually authenticated TLS. The HMAC key is unique per node, at least 256 bits, and never shared between nodes. The agent rejects unknown envelope and payload fields, oversized bodies, stale timestamps, wrong node IDs and unknown actions.

## Initial action allowlist

- `START`, `STOP`, `RESTART`: empty payload only.
- `UPDATE_PROPERTIES`: strict allowlist of MOTD, difficulty, gamemode, max players, PvP, online mode, flight, view distance, simulation distance and spawn protection with bounded values.

Raw shell strings, arbitrary URLs, Docker arguments, host paths, environment-variable injection and user-defined executable commands are forbidden.

## Required before enabling dispatch

1. Connect the panel HTTP client and durable replay journal to a crash-safe polling loop. Persist acceptance before acknowledgement. A crash after journal fsync but before acknowledgement must never cause duplicate execution.
2. Implement active lease renewal and bounded result reporting in the polling loop. Keep retry and cancellation semantics explicit.
3. Add node credential rotation and revocation; retain mandatory mTLS certificate verification and per-node HMAC keys.
4. Add a typed runtime adapter that maps actions to fixed container APIs; never invoke a shell or accept user-controlled container identifiers/options.
5. Enforce server-to-node ownership, per-server resource boundaries, rate limits and audit events before dispatch.
6. Test duplicate delivery, clock skew, expired leases, node restarts, partial failures and compromised/incorrect node credentials.
7. Keep operations marked undispatched until a worker actually claims them.

No production dispatch should be enabled until every gate above is implemented and reviewed.
