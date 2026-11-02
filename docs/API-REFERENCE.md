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
- `GET /servers/:serverId` — retrieve an authorized server detail, node health summary, access role, permissions and reserved TCP port.\n- `GET /servers/:serverId/console?tail=100` — fetch a bounded Docker stdout/stderr tail through the assigned node over mTLS. Requires owner access or subuser `viewConsole` permission; each read is audited and rate-limited.
- `POST /servers` — create a server against an active entitlement and compatible enabled release; reserves RAM/disk/CPU capacity and a unique TCP port, then atomically queues a signed PROVISION_SERVER node job. Returns HTTP 202; the server is PENDING until the node reports successful container creation.

Server responses never expose node-agent credentials or the node's private agent endpoint. Subusers only see servers shared with their account.

## Minecraft software catalogue
- `GET /software/releases` — list enabled releases; optional filters: `software` and `minecraftVersion`.
- `POST /admin/software/releases` — admin-only curated release registration. Body includes `software`, `minecraftVersion`, `softwareVersion`, `javaMajor`, `dockerImage`, optional `downloadUrl`, `checksumSha256`, `recommended` and `enabled`.

The catalogue is empty until an administrator adds verified releases. Server provisioning currently accepts only digest-pinned itzg/minecraft-server images (SHA-256) for supported Java software. Verify the image digest before enabling a release.

## Admin plans
- `GET /admin/plans` — list all plans, including disabled plans.
- `POST /admin/plans` — create a plan; new plans default to disabled. Price is an integer in paise.
- `PATCH /admin/plans/:planId` — update plan details or enable/disable it. Plan changes are audited; plans are not deleted so entitlement history remains intact.

## Admin
- `GET /admin/nodes` — list nodes with configured capacity, reserved capacity, overhead and telemetry.
- `POST /admin/nodes` — register a disabled node using an HTTPS agent endpoint, optional public Minecraft address, bounded capacity and TCP allocation range; returns a random agent token once.
- `PATCH /admin/nodes/:nodeId` — update the public Minecraft DNS name/IP used in customer connection strings.
- `POST /admin/nodes/:nodeId/check` — manually check node mTLS connectivity and refresh telemetry.
- `POST /admin/nodes/:nodeId/enable` — enable a node only after a recent successful mTLS health check and encrypted credentials are present.
- `POST /admin/nodes/:nodeId/reservations` — reserve RAM and disk on an enabled, online node. Body: `{ "memoryMb": 1024, "diskMb": 4096, "idempotencyKey": "unique-operation-key-0001" }`. Reservations expire after 15 minutes unless consumed or released.
- `POST /admin/nodes/:nodeId/reservations/:reservationId/release` — release an active reservation.
- `POST /admin/nodes/:nodeId/disable` — disable a registered node.
- `POST /admin/nodes/:nodeId/rotate-credentials` — revoke the old node token immediately, issue a one-time replacement and disable the node pending reconfiguration.
- `GET /admin/nodes/:nodeId/uncertain-jobs` — read-only view of acknowledged jobs requiring operator review.

All admin endpoints require an authenticated ADMIN session. Capacity reservations serialize on the node row, subtract configured overhead, use a unique idempotency key and reject overcommit. Host telemetry is informational; it is not treated as guaranteed allocatable capacity. Server creation reserves capacity and ports transactionally. Runtime disk quotas require Docker overlay2 on XFS with project quotas enabled; the host firewall must allow the configured allocation range.

## Plugin, mod and modpack catalogue
- `GET /artifacts` — list enabled projects and enabled versioned files; optional filters: `kind`, `software`, `minecraftVersion`, `loader`.
- `POST /admin/artifacts/projects` — admin-only project registration from Modrinth or CurseForge with compatibility, source and license metadata.
- `POST /admin/artifacts/projects/:projectId/files` — admin-only version/file registration with provider CDN host validation and optional SHA-256/size metadata.
- `GET /admin/artifacts/projects` — admin-only full catalogue review, including disabled projects and files.
- `PATCH /admin/artifacts/projects/:projectId` — enable or disable a curated project.
- `PATCH /admin/artifacts/files/:fileId` — enable or disable an individual pinned file.

Catalogue records default to disabled. These endpoints store metadata only; they do not download or install artifacts.

## Server operations
- `POST /servers/:serverId/operations` — owner or permission-scoped subuser queues idempotent `START`, `STOP`, or `RESTART`. Returns HTTP 202 with `execution: "QUEUED_FOR_NODE_AGENT"`. `UPDATE_PROPERTIES` is rejected until the verified server-file adapter is implemented.
- `GET /servers/:serverId/operations` — owner or subuser with `viewConsole` permission reads the latest 50 operations.

Operation requests and provisioning jobs are persisted and audited in the durable node-job outbox. The opt-in node worker claims signed jobs only when the node is enabled and online. PROVISION_SERVER creates a digest-pinned Java Minecraft container, applies memory/CPU/disk limits and publishes the reserved port. START/STOP/RESTART execute only against a uniquely labelled server container. The worker is disabled by default; verify Docker XFS project quotas and host firewall rules before enabling customer traffic.

## Server subusers
- `GET /servers/:serverId/subusers` — owner-only list of collaborators and permissions.
- `POST /servers/:serverId/subusers` — owner-only add an existing active, email-verified account by email. Optional permissions: `viewConsole`, `power`, `manageFiles`, `manageBackups`, `manageSettings`.
- `PATCH /servers/:serverId/subusers/:subuserId` — owner-only update permissions.
- `DELETE /servers/:serverId/subusers/:subuserId` — owner-only revoke access.

Subusers cannot manage other subusers or change ownership. Power permission authorizes queued start/stop/restart requests; settings permission authorizes allowlisted property updates; console permission authorizes operation history viewing. File and backup permissions are stored for their future APIs. Free server owners can use subusers; these endpoints do not require a paid entitlement. All changes are audited.


## Customer server detail UI
- `/servers` — customer server list and entitlement-backed server creation form.
- `/servers/:serverId` — server resource summary, reserved port, permission-aware power controls, audited auto-refreshing log console and recent operation history. Interactive WebSocket input and properties updates remain pending.
- `/admin/nodes` — node registration, health check, activation, disable, credential rotation and uncertain-job inspection.
- `/admin/releases` — curated pinned Java Minecraft image release management.
