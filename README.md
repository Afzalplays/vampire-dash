# Vampire Dash

**VampireHosting open-source Minecraft hosting control panel.**

Vampire Dash is a self-hosted control-plane project under active development and is not production-ready.

## Stack
- Web: Next.js + TypeScript
- API: Fastify + TypeScript
- Database: PostgreSQL + Prisma
- Cache/queues: Redis (integration pending)
- Game runtime: opt-in Go node agent with digest-pinned Java Minecraft Docker provisioning and power controls
- Transport: HTTPS public edge and mandatory mTLS node-agent channel
- Billing: provider adapters; FamGateway is the initial default
- Email: SMTP-based OTP verification and notification templates

## Repository map
- `app/` — web dashboard, authentication and email verification screens
- `apps/api/` — API service, authentication, Prisma schema, billing and notification services
- `apps/node-agent/` — Go node-agent HTTPS/mTLS service
- `docs/` — installation, architecture, security, billing and operations guides
- `docker-compose.dev.yml` — local PostgreSQL and Redis services

## Quick development setup

```bash
cp apps/api/.env.example apps/api/.env
docker compose -f docker-compose.dev.yml up -d
npm install
npm run db:generate
npm run db:migrate --workspace @vampire-dash/api
npm run dev:api
# In another terminal
npm run dev
```

Web: http://localhost:3000 · API: http://127.0.0.1:4000/api/health

## Documentation
- Installation: docs/INSTALLATION.md · Configuration: docs/CONFIGURATION.md
- Deployment/HTTPS: docs/DEPLOYMENT.md · Security: docs/SECURITY.md
- Node agent: docs/NODE-AGENT.md · Server management: docs/SERVER-MANAGEMENT.md
- Email notifications: docs/EMAIL-NOTIFICATIONS.md · Authentication: docs/AUTHENTICATION.md
- Billing gateways: docs/BILLING-GATEWAYS.md · FamGateway: docs/BILLING-FAMGATEWAY.md
- Roadmap: docs/ROADMAP.md · Contributing: CONTRIBUTING.md

## Status and safety
Email OTP verification, SMTP templates, entitlement-backed server creation, transactional TCP allocations, signed node jobs, digest-pinned Java container provisioning, memory/CPU/disk limits, START/STOP/RESTART, and an audited console with bounded log viewing and private RCON command input are implemented behind an opt-in node agent. The node agent requires HTTPS+mTLS and a per-node token. Disk quotas require Docker overlay2 on XFS with project quotas; firewall rules and PKI enrollment are operator-managed. Server data currently resides in the container writable layer and can be lost if the container is deleted; deletion is not exposed. Interactive console input, file manager, backups/restore, safe persistent bind mounts, Bedrock, automated enrollment, MFA, CSRF protection, payment reconciliation and production session hardening remain incomplete. Never use this code for live customer hosting or payments until the security checklist and tests are complete.

Never commit API keys, environment files, private keys, customer data or production database dumps. Rotate any credential that has been exposed.

---
Copyright VampireHosting 2026 - 2027


## Automated installation

An interactive **node-agent bootstrap installer** is available at `https://dash.vampirenodes.in/install.sh` when the Next.js app is deployed with the `public/` assets. It installs only the node agent; it does not install the panel, provision certificates, configure firewall rules or enable job execution. Keep job execution disabled until the node storage-quota and firewall checks pass. The full panel installer, safe upgrade/rollback and automated PKI enrollment remain planned; see [docs/INSTALLER-SPEC.md](docs/INSTALLER-SPEC.md).
