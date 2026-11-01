# Node-agent job protocol (design gate)

## Current implementation status

The agent currently exposes read-only health and capacity telemetry. The signed
job envelope validator in `jobs.go` is a tested protocol foundation only:
there is no job HTTP endpoint, polling loop, database lease worker, replay store,
container runtime, or command execution. The panel must continue to report
server operations as `NOT_DISPATCHED`.

The API now has a durable `NodeAgentJob` outbox record per operation, bound to
one node, with a unique nonce, bounded retry count, lease token hash and lease
expiry fields. The API creates an outbox row in the same transaction as each accepted operation. The `worker:agent-jobs` recovery process requeues expired leases with bounded exponential backoff and marks exhausted jobs DEAD/operations FAILED. It does not claim, dispatch, or execute jobs; actual agent delivery remains disabled.

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

1. Persist agent-side nonce replay protection across restarts and atomically
   reject duplicate job IDs/nonces.
2. Implement a trusted dispatcher that atomically claims eligible PENDING jobs, issues short leases, and reconciles signed acknowledgements. Expired-lease recovery and bounded backoff are implemented; heartbeat, cancellation semantics and result reconciliation remain pending.
3. Authenticate node identity and rotate per-node HMAC keys; retain mandatory
   mTLS certificate verification.
4. Add a typed runtime adapter that maps actions to fixed container APIs; never
   invoke a shell or accept user-controlled container identifiers/options.
5. Enforce server-to-node ownership, per-server resource boundaries, rate
   limits and audit events before dispatch.
6. Test duplicate delivery, clock skew, expired leases, node restarts, partial
   failures and compromised/incorrect node credentials.
7. Keep operations marked undispatched until a worker actually claims them.

No production dispatch should be enabled until every gate above is implemented
and reviewed.
