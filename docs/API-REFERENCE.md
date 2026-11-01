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

## Customer servers
- `GET /servers` — list servers owned by the signed-in user or shared with them as a subuser.
- `GET /servers/:serverId` — retrieve an authorized server detail, node health summary, access role and permissions.

Server responses never expose node-agent credentials or the node's private agent endpoint. Subusers only see servers shared with their account.

## Minecraft software catalogue
- `GET /software/releases` — list enabled releases; optional filters: `software` and `minecraftVersion`.
- `POST /admin/software/releases` — admin-only curated release registration. Body includes `software`, `minecraftVersion`, `softwareVersion`, `javaMajor`, `dockerImage`, optional `downloadUrl`, `checksumSha256`, `recommended` and `enabled`.

The catalogue is empty until an administrator adds verified releases. Adding a catalogue record does not download, install or provision a server. Container image references and artifact metadata must be verified before enabling a release.

## Admin
- `GET /admin/nodes` — list nodes with configured capacity, reserved capacity, overhead and telemetry.
- `POST /admin/nodes` — register a disabled node using an HTTPS endpoint and bounded capacity values; returns a random agent token once.
- `POST /admin/nodes/:nodeId/check` — manually check node mTLS connectivity and refresh telemetry.
- `POST /admin/nodes/:nodeId/reservations` — reserve RAM and disk on an enabled, online node. Body: `{ "memoryMb": 1024, "diskMb": 4096, "idempotencyKey": "unique-operation-key-0001" }`. Reservations expire after 15 minutes unless consumed or released.
- `POST /admin/nodes/:nodeId/reservations/:reservationId/release` — release an active reservation.
- `POST /admin/nodes/:nodeId/disable` — disable a registered node.

All admin endpoints require an authenticated ADMIN session. Capacity reservations serialize on the node row, subtract configured overhead, use a unique idempotency key and reject overcommit. Host telemetry is informational; it is not treated as guaranteed allocatable capacity. Provisioning is not yet connected to reservations.

## Plugin, mod and modpack catalogue
- `GET /artifacts` — list enabled projects and enabled versioned files; optional filters: `kind`, `software`, `minecraftVersion`, `loader`.
- `POST /admin/artifacts/projects` — admin-only project registration from Modrinth or CurseForge with compatibility, source and license metadata.
- `POST /admin/artifacts/projects/:projectId/files` — admin-only version/file registration with provider CDN host validation and optional SHA-256/size metadata.
- `GET /admin/artifacts/projects` — admin-only full catalogue review, including disabled projects and files.
- `PATCH /admin/artifacts/projects/:projectId` — enable or disable a curated project.
- `PATCH /admin/artifacts/files/:fileId` — enable or disable an individual pinned file.

Catalogue records default to disabled. These endpoints store metadata only; they do not download or install artifacts.

## Server operations
- `POST /servers/:serverId/operations` — owner or permission-scoped subuser queues idempotent `START`, `STOP`, `RESTART`, or allowlisted `UPDATE_PROPERTIES`. Returns HTTP 202 with `execution: "NOT_DISPATCHED"`.
- `GET /servers/:serverId/operations` — owner or subuser with `viewConsole` permission reads the latest 50 operations.

Operation requests are persisted and audited, but are not executed yet. Only allowlisted server properties are accepted; arbitrary file paths, shell commands, URLs and container arguments are rejected. A node-agent dispatcher/worker must be deployed before these requests can change a running server.

## Server subusers
- `GET /servers/:serverId/subusers` — owner-only list of collaborators and permissions.
- `POST /servers/:serverId/subusers` — owner-only add an existing active, email-verified account by email. Optional permissions: `viewConsole`, `power`, `manageFiles`, `manageBackups`, `manageSettings`.
- `PATCH /servers/:serverId/subusers/:subuserId` — owner-only update permissions.
- `DELETE /servers/:serverId/subusers/:subuserId` — owner-only revoke access.

Subusers cannot manage other subusers or change ownership. Power permission authorizes queued start/stop/restart requests; settings permission authorizes allowlisted property updates; console permission authorizes operation history viewing. File and backup permissions are stored for their future APIs. Free server owners can use subusers; these endpoints do not require a paid entitlement. All changes are audited.
