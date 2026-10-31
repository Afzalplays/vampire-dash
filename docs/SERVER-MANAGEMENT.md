# Server management

Minecraft provisioning is planned and not yet active. A server create request must validate owner, entitlement, node health, region, available RAM/disk/CPU, port allocation, software and version before scheduling a job.

Target software catalog: Paper, Vanilla, Fabric, Forge, Purpur, NeoForge, Velocity, BungeeCord, Bedrock Dedicated Server and GeyserMC. Only expose software/version combinations supported by the installed node image catalog.

Resource limits must be enforced by Docker/cgroups on the node, not just displayed in the UI. Idle sleep and paid 24/7 entitlements must be enforced server-side. Expiry may suspend service under a disclosed policy but must not silently delete customer data.
