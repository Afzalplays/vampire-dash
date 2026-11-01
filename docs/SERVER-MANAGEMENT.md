# Server management

Server creation currently validates owner, entitlement, node health, software release, server slots, RAM/disk/CPU capacity and an available TCP port. A per-node configurable allocation range defaults to 25565-25664; the chosen port is reserved transactionally with a unique database constraint and returned in the server response. Actual container provisioning, Docker cgroup enforcement and firewall publication are not yet active.

Target software catalog: Paper, Vanilla, Fabric, Forge, Purpur, NeoForge, Velocity, BungeeCord, Bedrock Dedicated Server and GeyserMC. Only expose software/version combinations supported by the installed node image catalog.

Control-plane CPU, RAM and disk capacity are checked before allocation. Runtime enforcement is still required: memory and CPU limits must be enforced by Docker/cgroups and disk usage by a quota-capable filesystem, not just displayed in the UI. Idle sleep and paid 24/7 entitlements must be enforced server-side. Expiry may suspend service under a disclosed policy but must not silently delete customer data.
