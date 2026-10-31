# Security model

- Passwords: Argon2id; never store plaintext passwords.
- Sessions: random opaque tokens, store only token digests, HttpOnly/SameSite cookies and bounded expiry.
- Authorization: deny by default; enforce ownership and admin roles in API handlers. Registration creates USER only.
- MFA: TOTP enrollment and hashed one-time recovery codes are required before production admin access.
- Abuse controls: rate-limit authentication and payment endpoints; add CAPTCHA where configured.
- Secrets: environment/secret manager only. Never commit .env, private keys or provider credentials.
- Billing: signed raw-body webhook validation, server-side verification, idempotency and immutable ledger.
- Nodes: authenticated agent identity, least privilege, resource limits and per-server filesystem isolation.
- Files: canonicalize paths and reject traversal/symlink escapes; enforce upload limits.
- Operations: audit admin actions and security-sensitive account changes.

The starter is not production-ready. CSRF protection, MFA, OAuth, webhook handling, node agent and security tests remain incomplete. Do not deploy for live customers.
