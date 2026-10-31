# Security requirements

- Argon2id password hashing; secure HttpOnly SameSite cookies.
- Server-side authorization on every user and admin API operation. New registrations receive USER role only.
- TOTP MFA and hashed one-time recovery codes.
- Rate limits for authentication and payment routes.
- Provider and node credentials stay server-side; never commit secrets.
- Signed webhooks, server-side payment verification, idempotency and immutable wallet ledger.
- Docker/cgroup resource enforcement and per-server filesystem isolation.
- No arbitrary host shell execution through the panel API.
- Audit privileged admin actions.

These controls are requirements, not yet implemented features. Do not use this scaffold for production.
