# Development installation

## Requirements
- Node.js 20.9 or newer and npm
- Docker Engine + Compose plugin
- Git

## Setup

```bash
git clone https://github.com/Afzalplays/vampire-dash.git
cd vampire-dash
cp apps/api/.env.example apps/api/.env
# Change SESSION_SECRET to a unique random value before starting services.
docker compose -f docker-compose.dev.yml up -d
npm install
npm run db:generate
npm run db:migrate --workspace @vampire-dash/api
npm run dev:api
```

In another terminal, run npm run dev and open http://localhost:3000. API health: http://127.0.0.1:4000/api/health.

## Notes
The API currently provides starter email/password routes and database health checks. Production deployment, OAuth, MFA, CSRF protection, full billing workflows, node provisioning and customer server operations are not complete. Do not expose the API publicly or accept customer payments with this development scaffold.
