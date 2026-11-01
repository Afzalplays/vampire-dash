# Roadmap

## Foundation
- [x] Next.js dashboard and authentication screens
- [x] Fastify API foundation and Prisma data model
- [x] Email OTP verification foundation
- [x] SMTP notification templates and expiry reminder worker
- [x] Go node-agent liveness and authenticated identity foundation
- [x] Node-agent read-only host CPU, memory and data-filesystem capacity endpoint (mTLS + bearer token)

## Core platform
- [x] Admin node registration, one-time bearer-token issuance, audit logging and disable endpoint (mTLS enrollment/rotation still pending)
- [x] AES-256-GCM encrypted storage for node-agent bearer credentials
- [x] Panel-side mTLS client for authenticated read-only node telemetry and admin health checks
- [x] Scheduled node health polling with PostgreSQL advisory-lock coordination and transition-only audit events
- [x] Node capacity reservation API with serialized overcommit checks, configurable overhead, expiry and idempotency
- [ ] Automated node enrollment/rotation
- [ ] Connect reservations to customer server creation and server lifecycle transitions
- [x] Durable idempotent server-operation queue schema for software and utility actions
- [x] Owner and permission-scoped subuser operation request/history API (queue only; dispatch pending)
- [x] Customer server creation API with active entitlement, slot/resource quota enforcement and healthy-node capacity selection (provisioning dispatch pending)
- [x] Typed signed node-job envelope validation (no endpoint, replay store, lease or execution yet)
- [ ] Persisted replay protection, job leases and secure panel-to-agent dispatch
- [ ] Operation worker, retries, cancellation and reconciliation
- [ ] Docker provisioning with per-server isolation and strict resource limits
- [ ] Server lifecycle state machine and reconciliation worker
- [ ] Port allocation and firewall integration
- [ ] WebSocket console with authorization and audit logging
- [ ] File manager with path confinement and upload limits
- [ ] Backup, restore and scheduled retention

## Customer and admin experience
- [x] Owner-managed server subusers with granular permissions and audited access revocation (available to free server owners)
- [x] Customer server list view backed by server and entitlement APIs; detail view pending
- [ ] Admin node, allocation, user and server management
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
- [x] Customer-facing queued operation history and controls (execution pending secure worker)

## Billing and security
- [ ] FamGateway webhook signature verification and payment reconciliation
- [ ] Durable email queue, retry/backoff and delivery monitoring
- [ ] MFA, CSRF protection, session revocation and security integration tests
- [ ] End-to-end tests and production deployment hardening

## Release
- [ ] Installer, service units and upgrade/rollback tooling (last stage)
