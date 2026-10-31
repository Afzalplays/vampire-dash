# Authentication implementation notes

The API includes email/password registration and login using Argon2id hashes and random opaque session tokens. Only a SHA-256 digest of each session token is stored in PostgreSQL. Registration and login have endpoint-specific rate limits. New registrations are always assigned USER by the server.

Email verification is required before a session is issued. A random 6-digit OTP is sent through the configured SMTP server, only its SHA-256 digest is stored, it expires after 10 minutes, and verification is limited to five attempts. Resend and verify endpoints are rate limited.

Still required before production: CSRF tokens for cookie-authenticated mutations, password reset, Google OAuth with state/nonce validation, TOTP MFA enrollment/verification and hashed recovery codes, session rotation/revocation controls, account abuse monitoring and security integration tests. Admin role assignment must be an audited out-of-band operation, never a registration field.
