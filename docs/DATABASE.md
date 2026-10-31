# Database

The API uses PostgreSQL with Prisma. Schema source: `apps/api/prisma/schema.prisma`.

Current starter models include users, sessions, wallets, immutable-style ledger entries, payment orders, Minecraft nodes and Minecraft servers. Before production, add migrations, integration tests, stronger database constraints and a reviewed ledger transaction service. Never edit production data manually without a backup and audit trail.

Development commands:

```bash
docker compose -f docker-compose.dev.yml up -d
npm run db:generate
npm run db:migrate --workspace @vampire-dash/api
```
