# Vampire Dash

**VampireHosting open-source Minecraft hosting control panel.**

Vampire Dash is a self-hosted control-plane project under active development and is not production-ready.

## Stack
- Web: Next.js + TypeScript
- API: Fastify + TypeScript
- Database: PostgreSQL + Prisma
- Cache/queues: Redis (integration pending)
- Game runtime: Docker-based node agent (planned)
- Billing: provider adapters; FamGateway is the initial default
- Email: SMTP-based OTP verification and notification templates

## Repository map
- app/ — web dashboard, authentication and email verification screens
- apps/api/ — API service, authentication, Prisma schema, billing and notification services
- docs/ — installation, architecture, security, billing and operations guides
- docker-compose.dev.yml — local PostgreSQL and Redis services

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
- Architecture: docs/ARCHITECTURE.md · Database: docs/DATABASE.md
- Plugins/software: docs/PLUGINS-AND-SOFTWARE.md · Plans/quotas: docs/PLANS-AND-QUOTAS.md
- Deployment: docs/DEPLOYMENT.md · Operations: docs/OPERATIONS.md
- Roadmap: docs/ROADMAP.md
- Authentication: docs/AUTHENTICATION.md · Email notifications: docs/EMAIL-NOTIFICATIONS.md
- API reference: docs/API-REFERENCE.md · Billing gateways: docs/BILLING-GATEWAYS.md
- FamGateway: docs/BILLING-FAMGATEWAY.md · Security: docs/SECURITY.md
- Node agent: docs/NODE-AGENT.md · Server management: docs/SERVER-MANAGEMENT.md
- Backups: docs/BACKUPS.md · Admin guide: docs/ADMIN-GUIDE.md
- User guide: docs/USER-GUIDE.md · Troubleshooting: docs/TROUBLESHOOTING.md
- Contributing: CONTRIBUTING.md

## Status and safety
Email OTP verification and SMTP notification templates are implemented as a foundation. Automated expiry scanning, durable retry workers and lifecycle event wiring remain pending. The web dashboard is a preview. MFA, OAuth, CSRF protection and production session hardening remain incomplete. Payment adapters are not enabled for customer use. Minecraft provisioning, node agent, console, file manager, backups and admin operations remain planned. Never use this code for live customer hosting or payments until the security checklist and tests are complete.

Never commit API keys, environment files, private keys, customer data or production database dumps. Rotate any credential that has been exposed.

---
Copyright VampireHosting 2026 - 2027
