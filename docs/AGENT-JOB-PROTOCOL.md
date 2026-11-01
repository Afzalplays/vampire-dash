# Node-agent job protocol (implementation status and design gate)

## Current implementation status

The panel API has a durable `NodeAgentJob` outbox record per operation, bound to one node, with a unique nonce, bounded retry count, lease-token hash and lease-expiry fields. Node-authenticated API endpoints implement claim, acknowledgement, lease renewal and result reporting. Claims use an atomic `FOR UPDATE SKIP LOCKED` selection and sign the exact serialized envelope with the per-node HMAC key. Lease tokens are random, stored only as SHA-256 hashes, and required for acknowledgement, renewal and result reporting. Queued payloads are validated against strict action schemas before signing.

The `worker:agent-jobs` recovery process retries expired pre-acknowledgement leases with bounded backoff. Expired acknowledged leases fail closed rather than replaying potentially side-effecting work. The node-authenticated `/api/agent/jobs/:jobId/reconcile` endpoint can accept a locally persisted terminal result after an acknowledged lease is marked lost, but only when the server independently verifies the immutable operation identity and the job has not been superseded. Matching committed results are idempotent; conflicting results are rejected. The Go poller reconciles `RESULT_PENDING` records before claiming new work. `EXECUTION_STARTED` without a durable terminal result remains uncertain and is never replayed.

The Go agent has strict signed-envelope validation and an append-only, fsync-backed replay journal protected by a file lock. The journal stores a SHA-256 digest of immutable operation identity (job ID, node ID, nonce, immutable server ID, action and canonical payload), excluding lease timestamps. The exact envelope bytes are still HMAC-authenticated. Reissued leases with new timestamps are recognized as the same accepted operation; reuse of a job ID or nonce with different operation content fails closed.

The Go panel HTTPS client supports bounded claim, acknowledgement, renewal and result requests. It rejects non-HTTPS panel URLs, blocks redirects to prevent bearer-token disclosure, bounds response sizes and supports an optional private panel CA while retaining system trust roots.

A testable polling state machine now connects the panel client contract, signature validation, durable replay journal, acknowledgement, bounded result reporting and periodic lease renewal while an executor runs. It refuses to start without a concrete executor and a bounded renewal interval. It ACKs only after the replay record is fsynced; identical redelivery retries ACK without re-executing. Renewal failure cancels the executor context and is reported as a failed operation. The poller is not wired into agent startup, and no production executor exists yet.

The stable replay identity issue is resolved. One important delivery gap remains: a process failure after ACK but before result reporting needs durable pending-result recovery and a server-side reconciliation path. Until then, acknowledged jobs whose result cannot be reported fail closed; they are never automatically re-executed. Long-running operations also require active lease renewal.

The agent currently exposes read-only health and capacity telemetry. It has no Docker runtime adapter or production server-operation execution. Customer operations must remain undispatched. The detailed recovery contract is documented in [ACKNOWLEDGED-JOB-RECOVERY-DESIGN.md](./ACKNOWLEDGED-JOB-RECOVERY-DESIGN.md); it is a design specification, not an implemented capability.

## Envelope

The panel-to-agent job envelope contains:

- `id`: opaque job identifier (16-128 URL-safe characters)
- `nodeId`: exact registered node identity
- `serverId`: immutable panel-owned Minecraft server identity associated with the operation
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

1. Implement the durable monotonic agent journal and node-authenticated result reconciliation contract in [ACKNOWLEDGED-JOB-RECOVERY-DESIGN.md](./ACKNOWLEDGED-JOB-RECOVERY-DESIGN.md); keep uncertain operations fail-closed and never replay an acknowledged side effect.
2. Add integration tests for ACK timeouts, crash points, re-leases, duplicate delivery, clock skew, expired leases and partial failures.
3. Wire the poller's tested lease-renewal loop into production startup only after a real executor exists; add timing and failure-injection integration tests.
4. Add route-level database integration tests for reconciliation and credential rotation; retain mandatory mTLS certificate verification and per-node HMAC keys.
5. Implement a typed Docker runtime adapter that resolves the signed server ID through a panel-owned server/container mapping and maps allowlisted actions to fixed container APIs; never invoke a shell or accept user-controlled container identifiers/options.
6. Enforce server-to-node ownership, per-server resource boundaries, rate limits and audit events before dispatch.
7. Wire the polling loop into startup only after the runtime adapter and all preceding safety gates pass.

No production dispatch should be enabled until every gate above is implemented and reviewed.
