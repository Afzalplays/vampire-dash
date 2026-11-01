# Node agent deployment

This guide installs the Go node agent as a systemd service. It does not issue PKI certificates; deploy certificates from your private CA before starting the service.

## Prerequisites

- Debian 12/13 or Ubuntu 22.04/24.04 with Go 1.23+ for building.
- Docker Engine installed and running.
- A DNS name and valid TLS certificate for the node-agent HTTPS endpoint.
- A private CA certificate used by the agent to verify the panel's mTLS client certificate.
- A node registered in the Vampire Dash admin UI. Copy the one-time agent token and node ID.
- A Docker data filesystem that supports per-container writable-layer quotas. The current implementation requires Docker overlay2 on XFS with project quotas enabled (pquota or prjquota). If Docker does not apply HostConfig.StorageOpt.size, provisioning fails.
- Host firewall access for TCP 8081 from the panel API and the configured Minecraft allocation range (default TCP 25565-25664) from players.

## Install the service

Build from the repository:

    cd apps/node-agent
    go test ./...
    go vet ./...
    go build -trimpath -ldflags="-s -w" -o vampire-node-agent .
    sudo install -o root -g root -m 0755 vampire-node-agent /usr/local/bin/vampire-node-agent

Create the service directories and install the PKI files:

    sudo install -d -o root -g root -m 0700 /etc/vampire-node-agent
    sudo install -d -o root -g root -m 0700 /etc/vampire-node-agent/pki
    sudo install -d -o root -g root -m 0700 /var/lib/vampire-dash/agent

Copy the environment template, then fill in the exact node ID and one-time token from the admin UI. Keep job execution disabled until the storage quota and firewall checks pass:

    sudo install -o root -g root -m 0600 deploy/node-agent.env.example /etc/vampire-node-agent/agent.env
    sudoedit /etc/vampire-node-agent/agent.env

Install and start the service:

    sudo install -o root -g root -m 0644 deploy/systemd/vampire-node-agent.service /etc/systemd/system/vampire-node-agent.service
    sudo systemctl daemon-reload
    sudo systemctl enable --now vampire-node-agent
    sudo systemctl status vampire-node-agent --no-pager
    sudo journalctl -u vampire-node-agent -n 50 --no-pager

The service runs as root because access to the Docker socket is effectively root-equivalent. It is restricted by systemd filesystem/network settings and never exposes the Docker socket over HTTP. Do not run untrusted binaries as the agent or grant untrusted users access to Docker.

## Verify and activate the node

1. In the admin UI, open Admin / Nodes and register the node with its HTTPS endpoint, physical RAM/disk capacity, public Minecraft DNS/IP address and allocation port range.
2. Copy the returned node ID and one-time agent token into the node environment file.
3. Install the node TLS certificate, private key and panel-client CA at the configured paths.
4. Restart the agent and run Check mTLS in Admin / Nodes.
5. Only after a recent successful health check, press Enable.
6. Confirm the node reports ONLINE and enabled.

## Enable provisioning only after storage checks

The agent reports host capacity from AGENT_DATA_DIR. Ensure this directory resides on the same filesystem used by Docker's data root if the panel's disk capacity display is intended to represent Docker capacity. Do not point AGENT_DATA_DIR at /var/lib/docker: the worker secures its journal directory to mode 0700.

Check the Docker storage driver and mount options:

    docker info --format 'Storage driver: {{.Driver}}'
    docker info --format 'Docker root: {{.DockerRootDir}}'
    findmnt -no FSTYPE,OPTIONS "$(docker info --format '{{.DockerRootDir}}')"

The Docker root filesystem must be XFS with project quotas enabled. Verify a disposable container can be created with a 512 MiB storage limit and inspect that Docker reports the same limit. Do not enable customer job execution if the storage driver ignores or rejects the quota.

After verifying the node firewall allows TCP 8081 from the panel and the configured Minecraft port range from players, edit the environment file to set AGENT_ENABLE_JOB_EXECUTION=true and PANEL_API_URL=https://panel.example.com. Then restart the service.

New server requests create a signed PROVISION_SERVER job. The worker pulls only a SHA-256-pinned itzg/minecraft-server image, applies memory/CPU/disk limits, publishes the reserved TCP port and verifies the container before marking the server RUNNING. The worker is disabled by default; enabling it is an operator action.

## Current limitations

Only Java Minecraft software supported by the curated image is provisioned. Bedrock, console, file manager, backup/restore, server deletion and property updates are not active. Server data currently lives in the Docker container writable layer and is lost if that container is deleted. Do not manually delete customer containers.