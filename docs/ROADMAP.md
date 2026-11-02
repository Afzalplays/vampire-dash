# Roadmap

## Foundation
- [x] Next.js dashboard and authentication screens
- [x] Fastify API foundation and Prisma data model
- [x] Email OTP verification foundation
- [x] SMTP notification templates and expiry reminder worker
- [x] Go node-agent liveness and authenticated identity foundation
- [x] Node-agent read-only host CPU, memory and data-filesystem capacity endpoint (mTLS + bearer token)

## Core platform
- [x] Admin node registration, one-time bearer-token issuance, health check, guarded enable/disable, credential rotation and audit logging (automated mTLS enrollment pending)
- [x] AES-256-GCM encrypted storage for node-agent bearer credentials
- [x] Panel-side mTLS client for authenticated read-only node telemetry and admin health checks
- [x] Scheduled node health polling with PostgreSQL advisory-lock coordination and transition-only audit events
- [x] Node capacity reservation API with serialized overcommit checks, configurable overhead, expiry and idempotency
- [ ] Automated node enrollment, credential rotation and revocation
- [x] Durable idempotent server-operation queue schema for software and utility actions
- [x] Owner and permission-scoped subuser operation request/history API
- [x] Customer server creation API with active entitlement, slot limits, RAM/disk/CPU capacity checks, healthy-node selection, transactional TCP port allocation and signed provisioning outbox job
- [x] Typed signed node-job envelope validation with strict action payload schemas
- [x] Durable per-operation node-job outbox with unique nonce, bounded attempts, lease metadata and retry indexes
- [x] Node-authenticated job claim, acknowledgement, lease renewal and result API endpoints
- [x] Bounded agent-job lease recovery with retry backoff and fail-closed handling after acknowledgement
- [x] Agent-side HMAC envelope verification and fsync-backed persistent replay journal
- [x] Go panel HTTPS client for claim/ack/renew/result with redirect blocking, bounded responses and optional private CA trust
- [x] Opt-in Go polling worker startup with durable replay journals and bounded configuration (disabled by default)
- [x] Resolve replay identity across re-leases and acknowledgement timeouts; add durable pending-result recovery
- [x] Wire polling into agent startup behind an explicit opt-in flag and strict prerequisites
- [x] Typed Docker adapter for digest-pinned Java Minecraft provisioning, per-server labels, memory/CPU/disk limits and power operations (overlay2/XFS project quota required)
- [x] Active lease renewal during long-running execution
- [x] Signed PROVISION_SERVER/START/STOP/RESTART execution, lease renewal, durable result reconciliation (UPDATE_PROPERTIES pending)
- [x] Server status transitions for successful, failed and uncertain node-job outcomes, including recovery reconciliation\n- [ ] Periodic Docker-to-database lifecycle reconciliation and operator repair workflow
- [x] Transactional per-node TCP port allocation with configurable ranges and uniqueness constraints
- [x] Docker TCP port publication from transactional per-node allocations
- [ ] Host firewall configuration and verification
- [x] Bounded server console log tail via mTLS, immutable server/node labels, subuser permission checks and audited customer polling UI (WebSocket interactive input remains pending)
- [ ] File manager with path confinement and upload limits
- [ ] Backup, restore and scheduled retention

## Customer and admin experience
- [x] Owner-managed server subusers with granular permissions and audited access revocation
- [x] Customer server list and detail views backed by server APIs, including power controls, operation history and live log polling
- [x] Admin node management UI (registration, mTLS check, activation, disable, token rotation, uncertain-job inspection)
- [ ] Admin allocation, user and server management
- [x] Database-backed plan catalogue and customer entitlement read APIs
- [ ] Admin plan CRUD UI and plan purchase/renewal flow
- [x] Enforce entitlement quotas during server creation
- [x] Curated Minecraft software release catalogue API with admin registration and public filters
- [ ] Automated upstream release sync, runtime compatibility resolver and verified image allowlist
- [x] Plugin/mod/modpack provider project and pinned-file catalogue schema with admin registration and public compatibility filters
- [ ] Plugin installer with trusted provider metadata and checksums
- [ ] Mod installer for Fabric/Forge/NeoForge with dependency checks
- [ ] Modpack installer with pinned manifests and rollback
- [ ] Server utilities: properties, EULA, schedules, backups, logs, ports and diagnostics
- [x] Customer-facing server controls and operation history backed by the opt-in secure node worker

## Billing and security
- [ ] FamGateway webhook signature verification and payment reconciliation
- [ ] Durable email queue, retry/backoff and delivery monitoring
- [ ] MFA, CSRF protection, session revocation and security integration tests
- [ ] End-to-end tests and production deployment hardening

## Release
- [ ] Installer, service units and upgrade/rollback tooling (last stage)
