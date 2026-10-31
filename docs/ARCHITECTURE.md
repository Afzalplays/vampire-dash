# Architecture

## Components
- Web: Next.js + TypeScript, browser treated as untrusted.
- API: Fastify service for identity, authorization, billing and provisioning.
- Data: PostgreSQL through Prisma; Redis is reserved for queues, rate limits and transient coordination.
- Workers: future asynchronous provisioning, backup, idle-sleep and reconciliation jobs.
- Game nodes: future Docker runtime controlled by a restricted node agent.
- Console: future authenticated WebSocket stream with server-scoped commands.

## Trust boundaries
All ownership, roles, quotas, billing and server lifecycle decisions are enforced by the API and node runtime. The browser must never receive provider secrets, node tokens or host credentials. The API must not expose arbitrary host shell execution. Node filesystem operations must remain within a server-specific root.

## Delivery stages
1. Repository and web/API foundation.
2. Authentication, sessions, database migrations and security controls.
3. Payment adapters, webhook verification, ledger and reconciliation.
4. Node enrollment, Docker isolation, quotas and server lifecycle.
5. Console, files, backups, plugins and player operations.
6. Monitoring, recovery tests, deployment hardening and installer release.
