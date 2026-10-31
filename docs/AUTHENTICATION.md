# Authentication implementation notes

The API starter includes email/password registration and login using Argon2id hashes and random opaque session tokens. Only a SHA-256 digest of the session token is stored in PostgreSQL. New registrations are always assigned USER by the server.

Still required before production: CSRF tokens for cookie-authenticated mutations, email verification, password reset, Google OAuth with state/nonce validation, TOTP MFA enrollment/verification and hashed recovery codes, session rotation/revocation controls, and security integration tests. Admin role assignment must be an audited out-of-band operation, never a registration field.
