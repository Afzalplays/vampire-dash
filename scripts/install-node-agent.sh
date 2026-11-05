#!/usr/bin/env bash
set -Eeuo pipefail

fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }
[[ "${EUID}" -eq 0 ]] || fail "Run this installer as root."
command -v curl >/dev/null 2>&1 || fail "curl is required."
command -v tar >/dev/null 2>&1 || fail "tar is required."
command -v go >/dev/null 2>&1 || fail "Go 1.23+ is required; install Go before running this installer."
command -v systemctl >/dev/null 2>&1 || fail "systemd is required."

go_version="$(go env GOVERSION | sed 's/^go//')"
go_major="${go_version%%.*}"
go_minor="${go_version#*.}"; go_minor="${go_minor%%.*}"
(( go_major > 1 || (go_major == 1 && go_minor >= 23) )) || fail "Go 1.23+ is required; found go${go_version}."

: "${AGENT_NODE_ID:?Set AGENT_NODE_ID from Admin / Nodes}"
: "${AGENT_TOKEN:?Set AGENT_TOKEN to the one-time token from Admin / Nodes}"
: "${PANEL_API_URL:?Set PANEL_API_URL to the HTTPS panel API base URL}"
: "${AGENT_TLS_CERT:?Set AGENT_TLS_CERT to the agent TLS certificate path}"
: "${AGENT_TLS_KEY:?Set AGENT_TLS_KEY to the agent TLS private key path}"
: "${AGENT_CLIENT_CA:?Set AGENT_CLIENT_CA to the panel client CA path}"

[[ "${AGENT_NODE_ID}" =~ ^[A-Za-z0-9_-]{1,128}$ ]] || fail "AGENT_NODE_ID contains invalid characters."
[[ "${AGENT_TOKEN}" =~ ^[A-Za-z0-9_-]{40,64}$ ]] || fail "AGENT_TOKEN must be a 40-64 character base64url token."
[[ "${PANEL_API_URL}" =~ ^https://[A-Za-z0-9.-]+(:[0-9]+)?(/[^?#[:space:]]*)?$ ]] || fail "PANEL_API_URL must be an HTTPS URL without credentials, query or fragment."
for value in "$AGENT_NODE_ID" "$AGENT_TOKEN" "$PANEL_API_URL" "$AGENT_TLS_CERT" "$AGENT_TLS_KEY" "$AGENT_CLIENT_CA"; do
  [[ "$value" != *$'\n'* && "$value" != *$'\r'* ]] || fail "Environment values must not contain newlines."
done
for file in "$AGENT_TLS_CERT" "$AGENT_TLS_KEY" "$AGENT_CLIENT_CA"; do
  [[ "$file" = /* && -f "$file" ]] || fail "Required PKI file does not exist or is not an absolute path: $file"
done

AGENT_LISTEN="${AGENT_LISTEN:-127.0.0.1:8081}"
AGENT_DOCKER_SOCKET="${AGENT_DOCKER_SOCKET:-/run/docker.sock}"
AGENT_DATA_DIR="${AGENT_DATA_DIR:-/var/lib/vampire-dash/agent}"
AGENT_ENABLE_JOB_EXECUTION="${AGENT_ENABLE_JOB_EXECUTION:-false}"
AGENT_JOB_POLL_SECONDS="${AGENT_JOB_POLL_SECONDS:-5}"
AGENT_LEASE_RENEW_SECONDS="${AGENT_LEASE_RENEW_SECONDS:-20}"
VAMPIRE_DASH_REF="${VAMPIRE_DASH_REF:-main}"

[[ "$AGENT_LISTEN" =~ ^([^[:space:]]+):([0-9]{1,5})$ ]] || fail "AGENT_LISTEN must be host:port."
listen_port="${BASH_REMATCH[2]}"
(( 10#$listen_port >= 1 && 10#$listen_port <= 65535 )) || fail "AGENT_LISTEN port must be between 1 and 65535."
[[ "$AGENT_DOCKER_SOCKET" = /* ]] || fail "AGENT_DOCKER_SOCKET must be an absolute path."
[[ "$AGENT_DATA_DIR" = /* && "$AGENT_DATA_DIR" != "/" ]] || fail "AGENT_DATA_DIR must be an explicit absolute non-root path."
[[ "$AGENT_ENABLE_JOB_EXECUTION" = "true" || "$AGENT_ENABLE_JOB_EXECUTION" = "false" ]] || fail "AGENT_ENABLE_JOB_EXECUTION must be true or false."
[[ "$AGENT_JOB_POLL_SECONDS" =~ ^([1-9]|[1-5][0-9]|60)$ ]] || fail "AGENT_JOB_POLL_SECONDS must be between 1 and 60."
[[ "$AGENT_LEASE_RENEW_SECONDS" =~ ^([1-9]|[12][0-9]|30)$ ]] || fail "AGENT_LEASE_RENEW_SECONDS must be between 1 and 30."

install -d -o root -g root -m 0700 /etc/vampire-node-agent
install -d -o root -g root -m 0700 /var/lib/vampire-dash/agent
install -d -o root -g root -m 0700 /etc/vampire-node-agent/pki

env_file=/etc/vampire-node-agent/agent.env
if [[ -e "$env_file" ]]; then
  fail "$env_file already exists. Preserve it and perform an explicit manual upgrade; this installer will not overwrite node credentials."
fi

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
archive="$tmp/source.tar.gz"
source_url="https://github.com/Afzalplays/vampire-dash/archive/${VAMPIRE_DASH_REF}.tar.gz"
printf 'Downloading Vampire Dash source ref %s...\n' "$VAMPIRE_DASH_REF"
curl --fail --silent --show-error --location --proto '=https' --tlsv1.2 "$source_url" -o "$archive"
mkdir -p "$tmp/source"
tar -tzf "$archive" | grep -Eq '(^|/)\.\./' && fail "Source archive contains an unsafe path."
tar -xzf "$archive" -C "$tmp/source" --strip-components=1

cd "$tmp/source/apps/node-agent"
go test ./...
go vet ./...
go build -trimpath -ldflags="-s -w" -o "$tmp/vampire-node-agent" .
install -o root -g root -m 0755 "$tmp/vampire-node-agent" /usr/local/bin/vampire-node-agent
install -o root -g root -m 0644 "$tmp/source/deploy/systemd/vampire-node-agent.service" /etc/systemd/system/vampire-node-agent.service

umask 077
cat > "$env_file" <<EOF
AGENT_LISTEN=$AGENT_LISTEN
AGENT_NODE_ID=$AGENT_NODE_ID
AGENT_TOKEN=$AGENT_TOKEN
AGENT_TLS_CERT=$AGENT_TLS_CERT
AGENT_TLS_KEY=$AGENT_TLS_KEY
AGENT_CLIENT_CA=$AGENT_CLIENT_CA
AGENT_ENABLE_JOB_EXECUTION=false
PANEL_API_URL=$PANEL_API_URL
AGENT_DOCKER_SOCKET=$AGENT_DOCKER_SOCKET
AGENT_DATA_DIR=$AGENT_DATA_DIR
AGENT_JOB_POLL_SECONDS=$AGENT_JOB_POLL_SECONDS
AGENT_LEASE_RENEW_SECONDS=$AGENT_LEASE_RENEW_SECONDS
EOF
chown root:root "$env_file"
chmod 0600 "$env_file"

systemctl daemon-reload
systemctl enable --now vampire-node-agent
systemctl --no-pager --full status vampire-node-agent || {
  journalctl -u vampire-node-agent -n 60 --no-pager || true
  fail "Node agent service did not start."
}

cat <<'EOF'
Node agent installed. Job execution remains DISABLED.
Next:
1. In Admin / Nodes, run Check mTLS.
2. Set the node's public Minecraft address and save it.
3. Verify Docker overlay2/XFS project quotas and open the TCP allocation range in the host firewall.
4. Only then edit /etc/vampire-node-agent/agent.env, set AGENT_ENABLE_JOB_EXECUTION=true, and restart the service.
EOF
