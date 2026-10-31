# Node agent

The node agent is not implemented yet. Planned responsibilities: secure node enrollment, health reporting, Docker container lifecycle, bounded log streaming, file operations within a server root, backup orchestration and resource telemetry.

Security requirements: per-node identity, credential rotation/revocation, mutually authenticated TLS, allow-listed typed operations, no arbitrary shell API, path traversal/symlink protections, per-server Docker isolation, strict CPU/RAM/disk/PID limits and audit logs. A panel API must never send raw shell strings to a node.
