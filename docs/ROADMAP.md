# Roadmap

## Foundation
- [x] Next.js dashboard and authentication screens
- [x] Fastify API foundation and Prisma data model
- [x] Email OTP verification foundation
- [x] SMTP notification templates and expiry reminder worker
- [x] Go node-agent liveness and authenticated identity foundation

## Core platform
- [ ] Panel-side node enrollment, credential rotation and health persistence
- [ ] Node telemetry: CPU, memory, disk and Docker capacity
- [ ] Docker provisioning with per-server isolation and strict resource limits
- [ ] Server lifecycle state machine and reconciliation worker
- [ ] Port allocation and firewall integration
- [ ] WebSocket console with authorization and audit logging
- [ ] File manager with path confinement and upload limits
- [ ] Backup, restore and scheduled retention

## Customer and admin experience
- [ ] Customer server list and server detail views backed by API
- [ ] Admin node, allocation, user and server management
- [ ] Plan catalogue and quota enforcement
- [ ] Plugin/modpack/software installation workflows

## Billing and security
- [ ] FamGateway webhook signature verification and payment reconciliation
- [ ] Durable email queue, retry/backoff and delivery monitoring
- [ ] MFA, CSRF protection, session revocation and security integration tests
- [ ] End-to-end tests and production deployment hardening

## Release
- [ ] Installer, service units and upgrade/rollback tooling (last stage)
