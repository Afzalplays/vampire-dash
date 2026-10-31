# Operations

Operational runbooks are still being developed. Production requirements include service health/readiness probes, structured logs with secret redaction, metrics, alerting, database backup verification, queue retry/dead-letter handling, node credential rotation, capacity monitoring and incident response.

Do not log passwords, session tokens, API keys, full payment payloads or customer file contents. Record security-relevant admin actions in the audit log.

## Capacity reservation expiry

Run `npm run worker:reservations` as a dedicated, supervised process alongside the API. It checks every 30 seconds, expires timed-out ACTIVE reservations, takes a row lock per node, recomputes cached reserved RAM/disk totals and writes one audit event per node expiry batch. Multiple worker instances are safe because node-row locking serializes reconciliation. Restart the worker automatically and alert if it remains stopped; the API's admission check still evaluates reservation rows and does not trust cached counters.
