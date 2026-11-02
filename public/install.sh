#!/usr/bin/env bash
set -Eeuo pipefail

fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
[[ "${EUID}" -eq 0 ]] || fail "Run with: curl -fsSL https://dash.vampirenodes.in/install.sh | sudo bash"
command -v curl >/dev/null 2>&1 || fail "curl is required."

printf '\nVampire Dash Node Agent installer\n'
printf 'Register the node first in Admin / Nodes, then enter its one-time credentials.\n\n'

read -r -p "Node ID: " AGENT_NODE_ID
read -r -s -p "One-time node token: " AGENT_TOKEN
printf '\n'
read -r -p "Panel API URL [https://dash.vampirenodes.in]: " PANEL_API_URL
PANEL_API_URL="${PANEL_API_URL:-https://dash.vampirenodes.in}"
read -r -p "Agent HTTPS listen address [0.0.0.0:8081]: " AGENT_LISTEN
AGENT_LISTEN="${AGENT_LISTEN:-0.0.0.0:8081}"
read -r -p "Agent TLS certificate path [/etc/vampire-node-agent/pki/agent.crt]: " AGENT_TLS_CERT
AGENT_TLS_CERT="${AGENT_TLS_CERT:-/etc/vampire-node-agent/pki/agent.crt}"
read -r -p "Agent TLS private key path [/etc/vampire-node-agent/pki/agent.key]: " AGENT_TLS_KEY
AGENT_TLS_KEY="${AGENT_TLS_KEY:-/etc/vampire-node-agent/pki/agent.key}"
read -r -p "Panel client CA path [/etc/vampire-node-agent/pki/panel-client-ca.crt]: " AGENT_CLIENT_CA
AGENT_CLIENT_CA="${AGENT_CLIENT_CA:-/etc/vampire-node-agent/pki/panel-client-ca.crt}"

[[ "${AGENT_NODE_ID}" =~ ^[A-Za-z0-9_-]{1,128}$ ]] || fail "Invalid node ID."
[[ "${AGENT_TOKEN}" =~ ^[A-Za-z0-9_-]{40,64}$ ]] || fail "Invalid one-time token."
[[ "${PANEL_API_URL}" =~ ^https://[A-Za-z0-9.-]+(:[0-9]+)?(/[^?#[:space:]]*)?$ ]] || fail "Panel API URL must be HTTPS."
[[ "${AGENT_LISTEN}" =~ ^[A-Za-z0-9.\[\]:_-]+:[0-9]{1,5}$ ]] || fail "Invalid listen address."
for file in "${AGENT_TLS_CERT}" "${AGENT_TLS_KEY}" "${AGENT_CLIENT_CA}"; do
  [[ "${file}" = /* && -f "${file}" ]] || fail "PKI file is missing or not an absolute path: ${file}"
done

export AGENT_NODE_ID AGENT_TOKEN PANEL_API_URL AGENT_LISTEN AGENT_TLS_CERT AGENT_TLS_KEY AGENT_CLIENT_CA
export AGENT_ENABLE_JOB_EXECUTION=false

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT
curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 \
  https://raw.githubusercontent.com/Afzalplays/vampire-dash/main/scripts/install-node-agent.sh -o "$tmp"
bash "$tmp"

unset AGENT_TOKEN
printf '\nInstaller finished. Job execution remains disabled until storage quotas and firewall rules are verified.\n'
