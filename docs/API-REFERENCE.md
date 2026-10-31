# API reference (starter)

Base URL in development: `http://127.0.0.1:4000/api`.

## Health
- `GET /health` — process health.
- `GET /health/ready` — database readiness check.

## Authentication
- `POST /auth/register` — JSON: `email`, `username`, `password`; creates USER role only.
- `POST /auth/login` — JSON: `email`, `password`; sets an HttpOnly session cookie.
- `GET /auth/me` — returns the authenticated user from the session cookie.
- `POST /auth/logout` — revokes the current session and clears the cookie.

Cookie-authenticated write endpoints require CSRF protection before production exposure. MFA, email verification, password reset and Google OAuth are not implemented yet. Do not treat this starter API as production-ready.
