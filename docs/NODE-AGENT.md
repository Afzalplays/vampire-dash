# Vampire Dash Node Agent

The node agent is a separate Go service installed on each Minecraft compute node.
This first implementation is a **control-plane connectivity foundation**. It does
not create containers, mount customer files, or expose Docker.

## Current capabilities

- Public liveness endpoint: `GET /health`.
- Token-authenticated node identity endpoint: `GET /v1/node/status`.
- Constant-time bearer-token comparison.
- HTTP read/write/idle timeouts and bounded request headers.
- No-store and nosniff response headers.
- Graceful shutdown on SIGINT/SIGTERM.
- Unit tests for liveness and authenticated status access.

## Build and run

Requires Go 1.23 or newer.

```bash
cd apps/node-agent
export AGENT_LISTEN=127.0.0.1:8081
export AGENT_NODE_ID='your-panel-node-id'
export AGENT_TOKEN="$(openssl rand -hex 32)"
go test ./...
go build -o vampire-node-agent .
./vampire-node-agent
```

The service reads environment variables directly; it intentionally does not load
dotenv files. Supply secrets through the service manager or a secret store.

## Endpoints

| Method | Path | Authentication | Purpose |
|---|---|---|---|
| GET | `/health` | None | Process liveness |
| GET | `/v1/node/status` | Bearer token | Authenticated node identity and agent version |

```bash
curl http://127.0.0.1:8081/health
curl -H "Authorization: Bearer $AGENT_TOKEN" http://127.0.0.1:8081/v1/node/status
```

## Network and secrets

- Keep the listener on loopback behind a TLS reverse proxy by default.
- Do not expose the agent directly to the public internet without TLS.
- Use a unique random token per node; do not reuse panel session secrets.
- Store the token in a root-readable environment file or secret manager, never Git.
- Rotate a disclosed token and update its panel-side credential.
- Restrict the reverse-proxy route to the panel API where practical.

## Not implemented

Docker provisioning, resource telemetry, server lifecycle operations, allocations,
WebSocket console, file transfer, backups, panel-side node enrollment and signed
request replay protection are not implemented. The current status response does
not prove that a node can safely host customer workloads. Do not connect this
agent to live customer servers until mutual authentication, authorization, Docker
isolation, quotas and integration tests are complete.
