# Minecraft software, plugins, mods, modpacks and utilities

## Software and version management

The supported software catalogue is planned to cover:
- Vanilla Minecraft
- Paper, Purpur and other Bukkit/Paper-compatible server distributions
- Fabric, Forge and NeoForge modded servers
- Velocity and BungeeCord proxy servers
- Bedrock Dedicated Server and GeyserMC where their runtime requirements are met

Version changes must be compatibility-checked against the selected software loader, Java runtime, installed plugins/mods and server data. A version change is a queued operation with a backup/recovery checkpoint; never overwrite a live server jar while its process is running. Keep the currently working version available for rollback.

## Software release catalogue foundation\n\nThe API now stores curated software releases with software family, Minecraft version, software version, Java major version, pinned container image reference, optional download URL and SHA-256 checksum, plus enabled/recommended flags. Public reads expose enabled entries only; admin creation is authenticated, validated and audited. The catalogue starts empty and no release is automatically trusted or installed. Upstream synchronization, compatibility resolution and image allowlisting remain required before production provisioning.\n\n## Plugin installer

The plugin installer will support compatible plugins for Paper/Spigot/Purpur-family servers. The catalogue must store provider, project ID, file ID, supported game versions, supported software, download URL, checksum and license/source metadata. Reject incompatible loaders and untrusted direct URLs. Install only into the isolated server's plugin directory.

## Mod installer

The mod installer will support Fabric, Forge and NeoForge according to each mod's loader and game-version metadata. Validate dependency requirements and conflicts before queueing installation. Never install a Fabric mod into Forge (or vice versa) merely because the Minecraft version matches.

## Modpack installer

Modpacks will be installed from an explicit, version-pinned manifest. Validate every referenced file, loader version, required Java version, source attribution and license metadata. Do not blindly execute arbitrary install scripts from uploaded archives. Import should be staged, scanned and switched atomically with rollback support.

## Server utilities

Planned customer utilities:
- Server properties editor with schema validation and safe restart
- EULA acceptance workflow with explicit customer confirmation
- Startup command and Java runtime selector constrained by supported images
- Scheduled restart controls
- Backup create, restore and retention controls
- World upload/import and safe archive extraction
- Server status, resource usage and player list
- Port/allocation viewer
- Log viewer and crash-report diagnostics
- Configuration editor for allowlisted files, with validation and backups

Utilities must be permission-checked per server, audited, rate-limited and executed through the node agent's authenticated job channel. Never expose the Docker socket or arbitrary host shell to the browser.

## Operation queue foundation

The database includes a durable, idempotent server-operation queue model for plugin/mod/modpack installation, version changes and common lifecycle/utility operations. This is a queue/data foundation only: a secure worker, node-agent command protocol, catalog integrations and execution handlers must be completed before these actions are exposed as working controls.

All downloaded artifacts require size limits, checksum verification, compatibility validation and isolated storage. The catalog and execution integrations are not yet production-ready.
