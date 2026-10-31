# Development roadmap

## Current foundation
- [x] Next.js dashboard preview and branding
- [x] Fastify API service foundation and environment validation
- [x] Prisma starter schema for users, sessions, wallets, payments, nodes and servers
- [x] Argon2id email/password registration and login starter
- [x] Server-side role helper and initial admin endpoints
- [x] FamGateway provider adapter contract and initial order/verification path

## Remaining before a secure beta
- [ ] CSRF protection, email verification, password reset and Google OAuth
- [ ] TOTP enrollment/verification, recovery codes and MFA enforcement
- [ ] Signed webhook endpoint with raw-body verification and idempotent event store
- [ ] Automated unit/integration tests and schema migrations committed
- [ ] Admin and user dashboard connected to API
- [ ] Node enrollment, agent, Docker isolation and resource enforcement
- [ ] Server lifecycle, console, files, backups and plugin management
- [ ] Monitoring, backup/restore drills, deployment hardening and security review

## Installer release gate
The installer must be the final deliverable after the supported deployment topology, required environment variables, migrations, service health checks, rollback behavior and upgrade strategy are documented and tested. It must never install an incomplete or insecure production system.
