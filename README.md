# Vampire Dash

**VampireHosting open-source Minecraft hosting control panel.**

Vampire Dash is a self-hosted control-plane project under active development and is not production-ready.

## Stack
- Web: Next.js + TypeScript
- API: Fastify + TypeScript
- Database: PostgreSQL + Prisma
- Cache/queues: Redis (integration pending)
- Game runtime: Go node-agent foundation; Docker orchestration pending
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
Email OTP verification and SMTP notification templates are implemented as a foundation. Automated expiry scanning is available as a separate worker; durable retry queues and lifecycle event wiring remain pending. The Go node agent requires HTTPS with mTLS and an additional per-node token, but Docker provisioning, resource enforcement, lifecycle, console, file manager and backups are not implemented. MFA, CSRF protection and production session hardening remain incomplete. Payment adapters are not enabled for customer use. Never use this code for live customer hosting or payments until the security checklist and tests are complete.

Never commit API keys, environment files, private keys, customer data or production database dumps. Rotate any credential that has been exposed.

---
Copyright VampireHosting 2026 - 2027
