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
- `GET /admin/nodes` — list nodes with configured capacity, reserved capacity, overhead and telemetry.
- `POST /admin/nodes` — register a disabled node using an HTTPS endpoint and bounded capacity values; returns a random agent token once.
- `POST /admin/nodes/:nodeId/check` — manually check node mTLS connectivity and refresh telemetry.
- `POST /admin/nodes/:nodeId/reservations` — reserve RAM and disk on an enabled, online node. Body: `{ "memoryMb": 1024, "diskMb": 4096, "idempotencyKey": "unique-operation-key-0001" }`. Reservations expire after 15 minutes unless consumed or released.
- `POST /admin/nodes/:nodeId/reservations/:reservationId/release` — release an active reservation.
- `POST /admin/nodes/:nodeId/disable` — disable a registered node.

All admin endpoints require an authenticated ADMIN session. Capacity reservations serialize on the node row, subtract configured overhead, use a unique idempotency key and reject overcommit. Host telemetry is informational; it is not treated as guaranteed allocatable capacity. Provisioning is not yet connected to reservations.
