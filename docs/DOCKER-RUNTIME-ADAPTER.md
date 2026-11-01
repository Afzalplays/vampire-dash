# Typed Docker runtime adapter

The Go node agent includes a typed Docker Engine adapter for desired-state power operations. It is intentionally not wired into the production polling loop.

## Security boundary

- Connects only to an absolute local Unix socket that exists and is not a symlink or regular file.
- Uses a pinned Docker Engine API version, bounded response bodies, request timeouts, and no HTTP redirects.
- Resolves the signed `serverId` using the exact `com.vampirenodes.server_id=<serverId>` Docker label.
- Requires exactly one matching container, validates the full Docker container ID, and re-checks the label through container inspect before acting.
- Never accepts a Docker container ID, host path, shell command, or Docker option from the job payload.
- Uses only fixed `GET /containers/json`, `GET /containers/{id}/json`, `POST /containers/{id}/start`, and `POST /containers/{id}/stop?t=30` endpoints.
- Verifies the resulting running state after each action.

## Supported and blocked operations

- `START`: ensure the mapped container is running; already-running is success.
- `STOP`: ensure the mapped container is stopped; already-stopped is success.
- `RESTART`: blocked until a durable restart generation/idempotency protocol exists.
- `UPDATE_PROPERTIES`: blocked until a typed, atomic, read-back-verified server-file adapter exists.

## Remaining requirements

The panel-owned provisioning path must create the container with the immutable server label and enforce the server-to-node mapping, image allowlist, memory/CPU/disk limits, network/port allocation, and filesystem isolation. Add integration tests against a disposable Docker daemon before considering startup wiring. The agent must remain undispatched until these gates pass.
