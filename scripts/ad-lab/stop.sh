#!/bin/bash
# Stage 45 / Stage 45b — Samba4 AD Lab Stop Script
# Stops and cleans up the disposable Samba4 AD DC

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yml"
CONTAINER_NAME="aether-ad-lab"

echo "[+] Stopping Samba4 AD Lab..."

# Stop and remove container
if docker ps --format '{{.Names}}' | grep -q "^aether-ad-lab$"; then
    echo "[+] Stopping container..."
    docker compose -f "${COMPOSE_FILE}" down
else
    echo "[.] Container not running"
fi

# Remove volumes (optional - comment out to preserve data)
echo "[+] Removing volumes..."
docker volume rm aether-ad-lab_samba_data aether-ad-lab_samba_etc 2>/dev/null || true

# Remove network
echo "[+] Removing network..."
docker network rm aether-ad-lab_aether-ad-net 2>/dev/null || true

echo "[+] Samba4 AD Lab stopped and cleaned up."