# Server management

Server creation validates owner, entitlement, node health, supported Java software release, server slots, RAM/disk/CPU capacity and an available TCP port. A per-node configurable allocation range defaults to 25565-25664; the chosen port is reserved transactionally and a signed `PROVISION_SERVER` job is written to the durable outbox. The opt-in node worker pulls a SHA-256-pinned `itzg/minecraft-server` image, creates a container, applies memory/CPU/disk limits and publishes the reserved port, then verifies the container before reporting success.

Target software catalog: Paper, Vanilla, Fabric, Forge, Purpur, NeoForge, Velocity, BungeeCord, Bedrock Dedicated Server and GeyserMC. Only expose software/version combinations supported by the installed node image catalog.

Control-plane CPU, RAM and disk capacity are checked before allocation. Runtime memory and CPU are enforced through Docker HostConfig. Disk enforcement uses Docker per-container `StorageOpt` and therefore requires overlay2 on XFS with project quotas enabled; provisioning fails closed when the requested quota is not applied. The host firewall must allow the configured allocation range. Idle sleep and paid 24/7 entitlements must be enforced server-side. Expiry may suspend service under a disclosed policy but must not silently delete customer data.


## Current provisioning limitations

Only Java Minecraft software supported by the curated image is provisioned; Bedrock is not enabled. Server files currently live in the container writable layer and survive restarts, but deleting the container would delete its data. Server deletion is not exposed. A per-server bind-mounted filesystem, console, file manager, backups/restore and properties updates remain future work. Keep `AGENT_ENABLE_JOB_EXECUTION=false` until the node's disk quota and firewall configuration have been verified.
