# Troubleshooting

## API does not start
- Check Node.js version (20.9+).
- Confirm apps/api/.env exists and DATABASE_URL is reachable.
- Confirm SESSION_SECRET is at least 32 characters.
- Start local PostgreSQL with docker compose -f docker-compose.dev.yml up -d.

## Prisma client missing
Run npm run db:generate from the repository root.

## Database migration fails
Check PostgreSQL health and DATABASE_URL. Back up persistent data before any destructive database operation.

## Feature is unavailable
This is an early development scaffold. MFA, OAuth, production billing, Minecraft node provisioning and customer server management are not complete.
