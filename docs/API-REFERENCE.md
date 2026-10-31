# API reference

Base path: `/api`.

## Authentication
- `POST /auth/register` — create an unverified account and send a 6-digit email OTP.
- `POST /auth/verify-email` — verify `{ "email": "...", "otp": "123456" }`; issues the session cookie after success.
- `POST /auth/resend-verification` — request another code for `{ "email": "..." }`.
- `POST /auth/login` — sign in; unverified accounts are rejected.
- `GET /auth/me` — current session.
- `POST /auth/logout` — revoke current session.

## Email preferences
- `GET /notifications/preferences` — read current user's email alert preferences.
- `PATCH /notifications/preferences` — update any of `emailEnabled`, `serverAlerts`, `billingAlerts`, `securityAlerts`.

Email event templates are documented in [EMAIL-NOTIFICATIONS.md](EMAIL-NOTIFICATIONS.md). SMTP credentials must remain in the API environment.

## Admin
- `GET /admin/nodes` — list registered nodes without credential material.
- `POST /admin/nodes` — register a disabled node using an HTTPS endpoint and bounded capacity values; returns a random agent token once.
- `POST /admin/nodes/:nodeId/disable` — disable a registered node.

All admin endpoints require an authenticated ADMIN session. Node registration and disable actions are audit logged. A newly registered node remains disabled until the mTLS enrollment and activation workflow is implemented.
