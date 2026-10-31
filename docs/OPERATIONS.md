# Operations

Operational runbooks are still being developed. Production requirements include service health/readiness probes, structured logs with secret redaction, metrics, alerting, database backup verification, queue retry/dead-letter handling, node credential rotation, capacity monitoring and incident response.

Do not log passwords, session tokens, API keys, full payment payloads or customer file contents. Record security-relevant admin actions in the audit log.
