# Installer design and release requirements

The project will ship a single entry-point installer only after application source, tests and operational documentation are complete. The installer is intentionally not included yet.

## Intended operator experience

A root/sudo operator will run one HTTPS-fetched command from the official Vampire Dash release. The installer must pin a tagged release and verify its checksum/signature before executing downloaded application code. It must never execute an unpinned `main` branch snapshot.

## Supported targets

- Debian 12 and Debian 13 (primary).
- Ubuntu 22.04 LTS and Ubuntu 24.04 LTS.
- Fresh installation first; upgrades are a separate, explicit operation.
- Unsupported distributions and architectures must fail before making system changes.

## Installation profiles

### Panel profile

- Install supported Node.js LTS, npm, PostgreSQL, Nginx, Certbot and required build tools.
- Create a dedicated unprivileged service account and application directories.
- Fetch a pinned Vampire Dash release and install locked npm dependencies.
- Generate high-entropy application secrets locally; never use sample secrets.
- Configure PostgreSQL with a dedicated least-privilege database role and random password.
- Run Prisma migrations, build the Next.js web app and Fastify API.
- Create hardened systemd units for web, API and scheduled workers.
- Configure Nginx reverse proxy and obtain Let's Encrypt certificates after DNS/port checks.
- Set production HTTPS origin and secure-cookie settings.
- Keep database and API ports private; do not expose PostgreSQL to the internet.

### Node profile

- Install Docker Engine from the official vendor repository and the pinned Go agent release.
- Create a dedicated agent account, systemd unit and restrictive filesystem permissions.
- Require node TLS certificate, trusted panel client CA and unique node token before enabling the agent.
- Do not expose Docker socket, host shell or agent management endpoints publicly.
- Configure firewall rules only after preserving the existing SSH port and validating the operator's current connection.

## Safety invariants

- Detect and preserve the current SSH port; never blindly reset SSH or UFW/nftables rules.
- Show planned firewall changes and require explicit confirmation before applying them.
- Never overwrite existing Nginx sites, certificates, database clusters or application data without a backup and explicit upgrade path.
- Use atomic config writes, restrictive permissions (secrets mode 0600), and rollback on failed deployment.
- Never disable TLS verification, generate placeholder production certificates, or silently fall back to HTTP.
- Validate DNS, public reachability, free disk/RAM, supported OS, architecture, time sync and required ports before installation.
- Never print passwords, API keys, bearer tokens or private keys to logs.
- Provide `--check`, `--dry-run`, `--panel`, `--node`, `--uninstall` and explicit upgrade modes.
- Uninstall must preserve customer data and database backups unless the operator explicitly requests deletion.
- Installation must be idempotent and log actions without logging secrets.

## Release gates before publishing installer

1. Application and Go agent CI green on supported versions.
2. Fresh-install tests on Debian 12/13 and Ubuntu 22.04/24.04.
3. Upgrade and rollback tests with existing data.
4. TLS issuance, renewal and expiry monitoring tests.
5. Firewall/SSH preservation tests, including non-standard SSH ports.
6. Backup and restore test with documented recovery.
7. Dependency and container image vulnerability scans.
8. Tagged release artifact, checksum and signature verification.

The installer is the final project deliverable, not a substitute for unfinished application features or deployment verification.
