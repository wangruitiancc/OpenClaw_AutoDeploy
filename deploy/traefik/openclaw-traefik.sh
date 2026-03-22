#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REMOTE_DIR="/opt/openclaw-traefik"
BASE_DOMAIN="${1:-}"

if [ -z "$BASE_DOMAIN" ]; then
    echo "Usage: $0 <base_domain>  (e.g., $0 api.example.com)"
    exit 1
fi

echo "=== Deploying OpenClaw Traefik Reverse Proxy ==="
echo "Base domain: $BASE_DOMAIN"

echo "Creating remote directory..."
ssh -p 32222 -i ~/.orbstack/ssh/id_ed25519 -o StrictHostKeyChecking=no wangruitian@ubuntu-24.04-desktop-vnc@127.0.0.1 "sudo mkdir -p $REMOTE_DIR && sudo chown wangruitian:wangruitian $REMOTE_DIR"

echo "Copying Traefik configuration..."
scp -P 32222 -i ~/.orbstack/ssh/id_ed25519 -o StrictHostKeyChecking=no \
    "$SCRIPT_DIR/docker-compose.yaml" \
    "$SCRIPT_DIR/traefik.yml" \
    "$SCRIPT_DIR/dynamic.yml" \
    wangruitian@ubuntu-24.04-desktop-vnc@127.0.0.1:"$REMOTE_DIR/"

echo "Starting Traefik container..."
ssh -p 32222 -i ~/.orbstack/ssh/id_ed25519 -o StrictHostKeyChecking=no wangruitian@ubuntu-24.04-desktop-vnc@127.0.0.1 bash -lc "cd $REMOTE_DIR && docker compose -f docker-compose.yaml up -d"

echo "Verifying Traefik is running..."
ssh -p 32222 -i ~/.orbstack/ssh/id_ed25519 -o StrictHostKeyChecking=no wangruitian@ubuntu-24.04-desktop-vnc@127.0.0.1 bash -lc "docker ps --format '{{.Names}}\t{{.Status}}' | grep traefik"

echo ""
echo "Traefik deployed successfully!"
echo "Dashboard: http://traefik.localtest.me:8090 (add to /etc/hosts: 127.0.0.1 traefik.localtest.me)"
echo ""
echo "Next steps:"
echo "1. Update /etc/openclaw-autodeploy/config.yaml:"
echo "   - Set runtime.base_domain: \"$BASE_DOMAIN\""
echo "   - Set runtime.docker_network: \"openclaw-proxy\""
echo "2. Restart ultraworker: sudo systemctl restart openclaw-ultraworker"
echo "3. Redeploy tenant containers to connect to openclaw-proxy network"
