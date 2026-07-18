#!/usr/bin/env bash
# Deploy shadow-llm-evaluator to a DigitalOcean Droplet over SSH.
#
# Usage:
#   ./scripts/deploy-droplet.sh root@143.244.131.169
#
# Builds a linux/amd64 binary locally (droplet is too small to compile),
# copies it + env files, installs a systemd service, opens port 8080.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
TARGET="${1:-}"
SSH_KEY="${SSH_KEY:-$HOME/.ssh/id_ed25519}"

if [[ -z "$TARGET" ]]; then
  echo "usage: $0 user@droplet-ip"
  exit 1
fi

REMOTE_DIR=/opt/shadow-llm-evaluator
BIN_NAME=shadow-llm-evaluator
BUILD_OUT="$ROOT_DIR/bin/${BIN_NAME}-linux-amd64"

echo "==> Cross-compiling linux/amd64 binary"
mkdir -p "$ROOT_DIR/bin"
(
  cd "$ROOT_DIR"
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o "$BUILD_OUT" .
)

echo "==> Preparing remote host"
ssh -i "$SSH_KEY" -o StrictHostKeyChecking=accept-new "$TARGET" bash -s <<'REMOTE'
set -euo pipefail
export DEBIAN_FRONTEND=noninteractive

# Swap helps on 512MB droplets if anything spikes.
if [[ ! -f /swapfile ]]; then
  fallocate -l 1G /swapfile || dd if=/dev/zero of=/swapfile bs=1M count=1024
  chmod 600 /swapfile
  mkswap /swapfile
  swapon /swapfile
  echo '/swapfile none swap sw 0 0' >> /etc/fstab
fi

id -u shadowllm >/dev/null 2>&1 || useradd --system --home /opt/shadow-llm-evaluator --shell /usr/sbin/nologin shadowllm
mkdir -p /opt/shadow-llm-evaluator/env
apt-get update -qq
apt-get install -y -qq ca-certificates curl ufw >/dev/null
ufw allow OpenSSH >/dev/null || true
ufw allow 8080/tcp >/dev/null || true
ufw --force enable >/dev/null || true
REMOTE

echo "==> Uploading binary + env files"
ssh -i "$SSH_KEY" "$TARGET" 'mkdir -p /tmp/shadow-llm-env'
scp -i "$SSH_KEY" "$BUILD_OUT" "$TARGET:/tmp/${BIN_NAME}"
scp -i "$SSH_KEY" -r "$ROOT_DIR/env/." "$TARGET:/tmp/shadow-llm-env/"
scp -i "$SSH_KEY" "$ROOT_DIR/deploy/shadow-llm-evaluator.service" "$TARGET:/tmp/shadow-llm-evaluator.service"
scp -i "$SSH_KEY" "$ROOT_DIR/deploy/shadow-llm-evaluator.env.example" "$TARGET:/tmp/shadow-llm-evaluator.env.example"

echo "==> Installing service"
ssh -i "$SSH_KEY" "$TARGET" bash -s <<'REMOTE'
set -euo pipefail
install -m 0755 /tmp/shadow-llm-evaluator /opt/shadow-llm-evaluator/shadow-llm-evaluator
rm -rf /opt/shadow-llm-evaluator/env
mkdir -p /opt/shadow-llm-evaluator/env
cp -a /tmp/shadow-llm-env/. /opt/shadow-llm-evaluator/env/
chown -R shadowllm:shadowllm /opt/shadow-llm-evaluator
install -m 0644 /tmp/shadow-llm-evaluator.service /etc/systemd/system/shadow-llm-evaluator.service
if [[ ! -f /etc/shadow-llm-evaluator.env ]]; then
  install -m 0600 /tmp/shadow-llm-evaluator.env.example /etc/shadow-llm-evaluator.env
fi
systemctl daemon-reload
systemctl enable --now shadow-llm-evaluator.service
systemctl --no-pager --full status shadow-llm-evaluator.service || true
sleep 1
curl -sS http://127.0.0.1:8080/healthz || true
echo
REMOTE

HOST_IP="${TARGET#*@}"
echo
echo "Deployed. Try:"
echo "  curl http://${HOST_IP}:8080/healthz"
echo "  curl -X POST http://${HOST_IP}:8080/v1/primary -H 'Content-Type: application/json' -d '{\"messages\":[{\"role\":\"user\",\"content\":\"hello\"}]}'"
echo
echo "To use DigitalOcean Inference on the droplet, edit /etc/shadow-llm-evaluator.env and set MODEL_ACCESS_KEY, then:"
echo "  ssh $TARGET 'systemctl restart shadow-llm-evaluator'"
