# Authentication implementation notes

The API includes email/password registration and login using Argon2id hashes and random opaque session tokens. Only a keyed HMAC-SHA-256 digest of each session token is stored in PostgreSQL. The same server-secret-keyed HMAC construction is used for email OTP digests, preventing a database-only leak from enabling offline OTP guessing. Digest comparisons use constant-time comparison after strict digest-format validation.

Registration and login have endpoint-specific rate limits. New registrations are always assigned USER by the server. Email verification is required before a session is issued. A random 6-digit OTP is sent through the configured SMTP server, expires after 10 minutes, and verification attempts are capped at five. Resend and verify endpoints are rate limited; failed attempts are incremented through a conditional database update to avoid exceeding the configured limit during concurrent requests.

Session cookies are HttpOnly and SameSite=Lax; production requires HTTPS and COOKIE_SECURE=true.

Still required before production: CSRF protection for cookie-authenticated mutations, password reset, Google OAuth with state/nonce validation, TOTP MFA enrollment/verification and hashed recovery codes, session rotation/revocation controls, account abuse monitoring and security integration tests. Admin role assignment must be an audited out-of-band operation, never a registration field.


**Upgrade note:** Switching session-token digests from unkeyed SHA-256 to keyed HMAC invalidates sessions created by earlier versions. Users must sign in again after deploying this change. No plaintext session tokens are available to migrate safely.
