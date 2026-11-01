# Typed Docker runtime adapter

The Go node agent includes a typed Docker Engine adapter for Java Minecraft provisioning and desired-state power operations. It is wired into the opt-in worker; the worker remains disabled by default.

## Security boundary

- Connects only to an absolute local Unix socket that exists and is not a symlink or regular file.
- Uses a pinned Docker Engine API version, bounded response bodies, request timeouts, and no HTTP redirects.
- Resolves the signed `serverId` using the exact `com.vampirenodes.server_id=<serverId>` Docker label.
- Requires exactly one matching container, validates the full Docker container ID, and re-checks the label through container inspect before acting.
- Never accepts a Docker container ID, host path, shell command, or Docker option from the job payload.
- Uses fixed Docker API endpoints for image pull, container creation, inspect, start, stop and restart; it never invokes a shell.
- Provisioning accepts only `itzg/minecraft-server` images pinned by SHA-256 digest.
- Applies Docker memory, NanoCPU, writable-layer `StorageOpt` size, host TCP port binding, PID limit, capability drop and `no-new-privileges`.
- Inspects the created container and verifies the server/node ownership labels, resource limits and host port before starting it.
- Stores server data in the container writable layer until a quota-backed per-server bind-mount implementation is available.
- Verifies the resulting running state after each action.

## Supported and blocked operations

- `START`: ensure the mapped container is running; already-running is success.
- `STOP`: ensure the mapped container is stopped; already-stopped is success.
- `RESTART`: stop then start the uniquely mapped container; the durable operation journal prevents blind replay after an uncertain crash.
- `UPDATE_PROPERTIES`: blocked until a typed, atomic, read-back-verified server-file adapter exists.

## Provisioning prerequisites and remaining work

Per-container `StorageOpt` disk quotas require Docker overlay2 on XFS with project quotas enabled. Provisioning fails closed if Docker rejects or does not report the requested quota. Operators must also open the configured TCP port range in the host firewall. The container writable layer is persistent across restarts but will be lost if the container is deleted; server deletion is therefore not exposed.

Still required: disposable-Docker integration tests, server-specific bind-mounted filesystem isolation, safe deletion, console, file manager, backups/restore, Bedrock support and a verified properties adapter. Keep the worker disabled until disk quotas and firewall rules are verified on each node.
