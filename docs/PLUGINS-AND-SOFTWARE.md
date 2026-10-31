# Minecraft software and plugin management

The planned software catalog includes Paper, Vanilla, Fabric, Forge, Purpur, NeoForge, Velocity, BungeeCord, Bedrock Dedicated Server and GeyserMC. Only publish combinations that have a validated, pinned runtime image and supported Java/runtime requirements.

Plugin and mod installation must validate file type, size, target software compatibility and checksum where available. Do not execute uploaded files on the host. Install into the server's isolated container volume and record actor, file, version and timestamp. Modpacks should use a curated manifest and explicit license/source metadata.

The catalog and installer are not implemented yet.
