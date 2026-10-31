# Configuration

Copy apps/api/.env.example to apps/api/.env. Required values include PostgreSQL DATABASE_URL and a unique SESSION_SECRET of at least 32 characters. Set COOKIE_SECURE=true behind HTTPS in production. WEB_ORIGIN must be the exact trusted web origin.

FamGateway credentials, when the integration is completed and reviewed, belong only in the API process environment as FAMGATEWAY_API_KEY. Never use a NEXT_PUBLIC_ prefix for secrets. Never commit environment files.
