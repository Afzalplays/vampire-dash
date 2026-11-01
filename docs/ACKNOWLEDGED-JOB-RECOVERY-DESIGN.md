# Acknowledged-job recovery design

## Status

Partially implemented. Production dispatch remains disabled. The existing fail-closed recovery behavior must remain in place until all safety gates are implemented and tested.

## Failure window

The agent durably reserves replay identity before ACK, then executes and reports a result. A crash after ACK can leave the panel unable to distinguish work that never started, partially completed work, completed work with a lost response, or a result that committed while its response was lost. Replaying an acknowledged operation is unsafe.

## Durable agent journal

Extend the append-only, fsync-backed journal to track monotonic states keyed by stable operation identity (job ID, node ID, nonce, immutable server ID, action, canonical payload digest):

1. ACCEPTED — signed envelope validated and identity reserved.
2. ACK_CONFIRMED — panel ACK response received.
3. EXECUTION_STARTED — intent persisted immediately before entering the runtime adapter.
4. RESULT_PENDING — exact bounded terminal result persisted before network submission.
5. RESULT_CONFIRMED — panel confirms the same result hash is committed.

Lease timestamps and lease tokens are not operation identity. Do not store lease tokens in plaintext in the operation journal. Corrupt, truncated, unknown, or contradictory records fail closed. Any compaction must atomically replace the journal, fsync the replacement and parent directory, and preserve unresolved records.

## Reconciliation API

Add a node-authenticated endpoint scoped to both authenticated node ID and job ID. The agent supplies its stable operation identity digest and journal state. For RESULT_PENDING it also supplies the result hash and bounded result. The API independently derives the expected digest from immutable server-side job fields and payload; it must not trust a node-supplied digest as proof.

Possible outcomes: RESULT_COMMITTED, RESULT_ACCEPTED, IN_PROGRESS, UNCERTAIN, CONFLICT, or NOT_FOUND. Reconciliation is idempotent for the same operation digest and result hash. A different result hash for a terminal job is a conflict. Update NodeAgentJob and MinecraftServerOperation in one transaction. Never reopen a DEAD acknowledged job for execution. Reconciliation may accept a matching terminal result, but must never authorize execution or replay.

## Runtime semantics

START and STOP are recoverable only with idempotent desired-state operations (ensure-running / ensure-stopped) against panel-owned server identities. UPDATE_PROPERTIES requires atomic application of a validated desired configuration and read-back verification. RESTART remains UNCERTAIN after interrupted execution unless the runtime adapter supports a durable operation generation/token. No user-controlled shell commands, host paths, container identifiers, or runtime options are allowed.

The executor persists EXECUTION_STARTED before the runtime call and RESULT_PENDING before network result submission. It never reruns work merely because result submission failed.

## Rollout gates

1. Implement the journal and fault-injection tests for each transition and corrupt/torn records.
2. Add a server-derived immutable operation digest and a migration with a safe backfill strategy.
3. Implement reconciliation with transaction, idempotency, and cross-node isolation tests.
4. Test lost ACK responses, crashes before and after runtime completion, lost result responses, duplicate reconciliation, conflicting result hashes, expired leases, and superseded operations.
5. Implement desired-state runtime operations for START, STOP, and properties; keep RESTART fail-closed until it has a true idempotency mechanism.
6. Run Go, TypeScript, Prisma migration, and integration suites in CI; review logs for credential and payload leakage.
7. Consider a limited canary only after every gate passes. Production dispatch remains disabled until then.


## Implementation status

The local Go agent now has an append-only, fsync-backed operation journal with ACCEPTED, ACK_CONFIRMED, EXECUTION_STARTED, RESULT_PENDING, and RESULT_CONFIRMED checkpoints. The worker persists each checkpoint around ACK, execution, and result submission. Pending terminal results are stored without the ephemeral lease token. A recovery reader validates the complete monotonic history and returns each job's latest state.

The poller fails closed and refuses to claim additional work while any journaled operation is not RESULT_CONFIRMED. This prevents new dispatch from hiding an unresolved operation, but it does not itself reconcile or resume that operation. A process restart with an unresolved record therefore requires the server-side reconciliation path below before polling can safely continue.

The node-authenticated reconciliation endpoint and the Go panel client are now implemented. The endpoint independently derives the immutable operation digest from the stored job and operation, scopes lookups to the authenticated node, and accepts a matching pending terminal result only after the recovery worker has marked an acknowledged lease lost. Reconciliation is idempotent for an already committed matching result and rejects identity/result conflicts. The Go poller retries RESULT_PENDING reconciliation after restart and only resumes polling after RESULT_ACCEPTED or RESULT_COMMITTED. EXECUTION_STARTED and other uncertain states remain blocked and are never replayed automatically. Cross-language Go/TypeScript hash compatibility tests cover HTML-sensitive strings and canonical object key order.

Implemented since the initial recovery milestone: administrator-triggered node credential rotation (with immediate revocation, node disablement, and an acknowledged-job guard), plus a read-only administrator endpoint to inspect uncertain jobs. Operator handling documentation is in `docs/NODE-CREDENTIAL-ROTATION.md` and `docs/UNCERTAIN-JOB-OPERATIONS.md`.

Still not implemented: route-level database integration tests for reconciliation and credential rotation, a typed production Docker runtime adapter, runtime integration/fault-injection tests, and production startup wiring. The agent remains undispatched in production.