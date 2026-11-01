# Node-agent job protocol (implementation status and design gate)

## Current implementation status

The panel API has a durable `NodeAgentJob` outbox record per operation, bound to one node, with a unique nonce, bounded retry count, lease-token hash and lease-expiry fields. Node-authenticated API endpoints implement claim, acknowledgement, lease renewal and result reporting. Claims use an atomic `FOR UPDATE SKIP LOCKED` selection and sign the exact serialized envelope with the per-node HMAC key. Lease tokens are random, stored only as SHA-256 hashes, and required for acknowledgement, renewal and result reporting. Queued payloads are validated against strict action schemas before signing.

The `worker:agent-jobs` recovery process retries expired pre-acknowledgement leases with bounded backoff. Expired acknowledged leases fail closed rather than replaying potentially side-effecting work.

The Go agent has strict signed-envelope validation and an append-only, fsync-backed replay journal protected by a file lock. The journal stores the SHA-256 digest of the exact signed body. A byte-identical redelivery is recognized as an already accepted duplicate; reuse of a job ID or nonce with different content fails closed.

The Go panel HTTPS client supports bounded claim, acknowledgement, renewal and result requests. It rejects non-HTTPS panel URLs, blocks redirects to prevent bearer-token disclosure, bounds response sizes and supports an optional private panel CA while retaining system trust roots.

A testable polling state machine now connects the panel client contract, signature validation, durable replay journal, acknowledgement and bounded result reporting. It refuses to start without a concrete executor. It ACKs only after the replay record is fsynced; identical redelivery retries ACK without re-executing. The poller is not wired into agent startup, and no production executor exists yet.

Important remaining delivery edge cases: a lease reclaimed with a newly serialized signed envelope may conflict with the journal's exact-body replay key, and a process failure after ACK but before result reporting needs durable local pending-result recovery. These must be resolved before enabling dispatch. Long-running operations also require active lease renewal.

The agent currently exposes read-only health and capacity telemetry. It has no Docker runtime adapter or production server-operation execution. Customer operations must remain undispatched.

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

1. Fix stable replay identity across lease renewals/reclaims and persist pending results so ACKed work can safely recover after a process restart.
2. Add integration tests for ACK timeouts, crash points, re-leases, duplicate delivery, clock skew, expired leases and partial failures.
3. Implement active lease renewal for long-running operations.
4. Add node credential rotation and revocation; retain mandatory mTLS certificate verification and per-node HMAC keys.
5. Implement a typed Docker runtime adapter that maps allowlisted actions to fixed container APIs; never invoke a shell or accept user-controlled container identifiers/options.
6. Enforce server-to-node ownership, per-server resource boundaries, rate limits and audit events before dispatch.
7. Wire the polling loop into startup only after the runtime adapter and all preceding safety gates pass.

No production dispatch should be enabled until every gate above is implemented and reviewed.
