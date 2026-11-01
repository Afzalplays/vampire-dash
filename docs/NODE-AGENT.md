# Vampire Dash Node Agent

The node agent is a separate Go service installed on each Minecraft compute node.
It provides secure HTTPS/mTLS connectivity, authenticated status and read-only host
capacity telemetry. An explicitly opt-in outbound worker can claim signed jobs, provision
Java Minecraft containers from digest-pinned official images, and perform START/STOP/RESTART
against containers with the panel-owned server and node labels. It never exposes Docker over HTTP.

## Security controls

- HTTPS is mandatory; there is no plaintext fallback.
- TLS 1.2 minimum (TLS 1.3 is negotiated when supported).
- Mutual TLS is mandatory: the agent verifies a client certificate issued by
  the configured panel-client CA.
- The panel must verify the agent certificate against its trusted node-agent CA.
- Status and metrics endpoints also require a unique per-node bearer token,
  compared in constant time.
- HTTP read/write/idle timeouts and bounded request headers.
- No-store and nosniff response headers.
- Graceful shutdown on SIGINT/SIGTERM.

## PKI layout

Use a dedicated private CA for node-agent server certificates and a separate
client certificate issued to the panel API. Keep CA private keys offline or in
a managed secrets system. Deploy only CA public certificates to the services.
Issue a unique server certificate for each node with its DNS name in the SAN.
Protect node and panel private keys with root-only access.

## Configuration

| Variable | Purpose |
|---|---|
| `AGENT_LISTEN` | Bind address; defaults to loopback |
| `AGENT_NODE_ID` | Panel-side node identifier |
| `AGENT_TOKEN` | Unique random token, minimum 32 characters |
| `AGENT_TLS_CERT` | Agent HTTPS certificate PEM |
| `AGENT_TLS_KEY` | Agent private key PEM |
| `AGENT_CLIENT_CA` | CA PEM used to verify panel client certificates |
| `AGENT_DATA_DIR` | Filesystem path whose disk capacity is reported; defaults to `/`. When job execution is enabled, this must be an explicit persistent non-root directory and is secured to mode `0700` |
| `AGENT_ENABLE_JOB_EXECUTION` | Set to `true` to opt in to outbound job polling; default `false` |
| `PANEL_API_URL` | HTTPS base URL of the Vampire Dash API; required when job execution is enabled |
| `AGENT_DOCKER_SOCKET` | Local Docker Unix socket; defaults to `/var/run/docker.sock` |
| `AGENT_JOB_POLL_SECONDS` | Poll interval from 1 to 60 seconds; defaults to 5 |
| `AGENT_LEASE_RENEW_SECONDS` | Lease renewal interval from 1 to 30 seconds; defaults to 20 |

## Build and run

Requires Go 1.23 or newer. Configure the PKI files first; startup fails closed if
any certificate, key or CA is missing or invalid.

```bash
cd apps/node-agent
export AGENT_LISTEN=127.0.0.1:8081
export AGENT_NODE_ID='your-panel-node-id'
export AGENT_TOKEN="$(openssl rand -hex 32)"
export AGENT_TLS_CERT=/etc/vampire-node-agent/pki/agent.crt
export AGENT_TLS_KEY=/etc/vampire-node-agent/pki/agent.key
export AGENT_CLIENT_CA=/etc/vampire-node-agent/pki/panel-client-ca.crt
export AGENT_DATA_DIR=/var/lib/vampire-dash/servers
go test ./...
go vet ./...
go build -o vampire-node-agent .
./vampire-node-agent
```

## Optional job worker

Job execution is disabled unless `AGENT_ENABLE_JOB_EXECUTION=true` is explicitly set. When enabled, the agent requires an HTTPS panel URL, a persistent non-root data directory, and an existing local Docker Unix socket. The panel bearer token is also used to verify the signed job envelope. The worker persists replay and operation journals before acknowledging jobs, renews leases while actions run, and refuses new work when an operation is uncertain.

The Docker adapter supports `PROVISION_SERVER`, desired-state START/STOP and stop/start RESTART. Provisioning accepts only `itzg/minecraft-server` images pinned by SHA-256 digest and allowlisted Java software. It publishes the reserved TCP port, applies Docker memory and NanoCPU limits, requests a per-container writable-layer disk quota with `HostConfig.StorageOpt`, drops Linux capabilities, enables `no-new-privileges`, and applies a PID limit. It inspects the created container and verifies ownership labels, limits and port mapping before starting it. If the Docker storage driver cannot enforce the requested disk quota, provisioning fails closed. Container data currently lives in the container writable layer; deleting the container would delete server data, so deletion is not exposed.

The worker remains disabled by default. Before enabling it for customers, configure Docker `overlay2` on XFS with project quotas (`pquota`/`prjquota`), verify per-container `StorageOpt` limits on the node, open the configured TCP allocation range in the host firewall, and validate the node with the admin mTLS health check. `BEDROCK`, console, file manager, backups, restore and `UPDATE_PROPERTIES` remain unavailable.

## Endpoints

All endpoints require HTTPS and a valid panel client certificate at the TLS
layer. Status and metrics additionally require the bearer token.

| Method | Path | Application auth | Response |
|---|---|---|---|
| GET | `/health` | mTLS only | Agent liveness/version |
| GET | `/v1/node/status` | mTLS + bearer | Node identity and agent version |
| GET | `/v1/node/metrics` | mTLS + bearer | CPU core count, host memory and configured data filesystem capacity |

Metrics are read-only snapshots. Memory values come from Linux `/proc/meminfo`;
disk values come from `statfs(AGENT_DATA_DIR)`. They are host-level capacity
values, not Docker quota accounting or per-container usage. The panel must not
treat them as a reservation guarantee.

Example from the panel API host (client certificate and CA are mandatory):

```bash
curl --cacert /etc/vampire-panel/pki/node-agent-ca.crt \
  --cert /etc/vampire-panel/pki/panel-client.crt \
  --key /etc/vampire-panel/pki/panel-client.key \
  -H "Authorization: Bearer $AGENT_TOKEN" \
  https://node.example.com:8081/v1/node/metrics
```

## Network and secrets

- Keep the listener on loopback or a private interface; firewall it to the panel API.
- Never expose the Docker socket or host shell through the agent.
- Use unique tokens and certificates per node; rotate and revoke on compromise.
- Store private keys and tokens in a root-only secret file or secret manager, never Git.
- Validate certificate SANs and expiry during panel-side TLS client setup.
- Do not disable certificate verification or use insecure-skip-verify.

## Remaining work

Remaining work: server-specific bind-mounted filesystem isolation and safe deletion, console, file manager, backup/restore, Bedrock runtime, verified properties adapter, automated panel-side enrollment and certificate issuance/rotation. Port allocation and Docker port publication are implemented; the host firewall range must still be configured by the operator. Keep the worker disabled until Docker disk quota support and firewall access are verified on each node.
