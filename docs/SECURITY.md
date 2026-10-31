# Security model

- Passwords: Argon2id; never store plaintext passwords.
- Sessions: random opaque tokens; store only keyed HMAC-SHA-256 digests; HttpOnly/SameSite cookies and bounded expiry.
- Email verification: 6-digit, 10-minute OTP; HMAC-SHA-256 digest keyed by SESSION_SECRET; constant-time digest comparison; five-attempt cap.
- Authorization: deny by default; enforce ownership and admin roles in API handlers. Registration creates USER only.
- Transport: HTTPS for public web/API, TLS 1.2 minimum, automatic certificate renewal and HTTP-to-HTTPS redirect.
- Node channel: agent requires mandatory mTLS with certificate verification plus a unique per-node bearer token; no plaintext fallback.
- Node registration: admin-only HTTPS endpoint, one-time random token delivery, keyed HMAC token storage, disabled-by-default state and audit logging.
- MFA: TOTP enrollment and hashed one-time recovery codes are required before production admin access.
- Abuse controls: rate-limit authentication and payment endpoints; add CAPTCHA where configured.
- Secrets: environment/secret manager only. Never commit .env, private keys or provider credentials.
- Billing: signed raw-body webhook validation, server-side verification, idempotency and immutable ledger.
- Nodes: authenticated agent identity, least privilege, resource limits and per-server filesystem isolation.
- Files: canonicalize paths and reject traversal/symlink escapes; enforce upload limits.
- Operations: audit admin actions and security-sensitive account changes.

The starter is not production-ready. CSRF protection, MFA, OAuth, webhook handling,
automated node enrollment, Docker isolation and security integration tests remain incomplete.
Do not deploy for live customers.
