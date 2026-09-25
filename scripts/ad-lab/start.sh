#!/bin/bash
# Stage 45 / Stage 45b — Samba4 AD Lab Start Script
# Starts the disposable Samba4 AD DC for live Kerberos qualification

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yml"
CONTAINER_NAME="aether-ad-lab"

echo "[+] Starting Samba4 AD Lab..."
echo "[+] Compose file: ${COMPOSE_FILE}"

# Check if container already exists
if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
    echo "[!] Container ${CONTAINER_NAME} already exists. Removing..."
    docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
fi

# Start the container
echo "[+] Bringing up container..."
docker compose -f "${COMPOSE_FILE}" up -d

# Wait for healthcheck to pass
echo "[+] Waiting for Samba4 to become healthy (max 120s)..."
timeout=120
elapsed=0
interval=5

while [ $elapsed -lt $timeout ]; do
    health=$(docker inspect --format='{{.State.Health.Status}}' aether-ad-lab 2>/dev/null || echo "none")
    if [ "$health" = "healthy" ]; then
        echo "[+] Samba4 is healthy!"
        break
    fi
    echo "[.] Waiting... (${elapsed}s/${timeout}s) health=${health}"
    sleep $interval
    elapsed=$((elapsed + interval))
done

if [ $elapsed -ge $timeout ]; then
    echo "[!] Timeout waiting for Samba4 to become healthy"
    echo "[!] Container logs:"
    docker logs aether-ad-lab --tail 50
    exit 1
fi

# Verify ports are listening
echo "[+] Verifying KDC and LDAP ports..."
sleep 3
docker exec aether-ad-lab netstat -tlnp | grep -E ':88|:389|:445|:464' || echo "[!] Warning: some ports not visible in netstat"

# Quick KDC test
echo "[+] Testing KDC connectivity..."
docker exec aether-ad-lab samba-tool domain info AETHER.TEST || echo "[!] samba-tool domain info failed"

echo "[+] Samba4 AD Lab is ready!"
echo "[+] Realm: AETHER.TEST"
echo "[+] Domain: AETHER"
echo "[+] DC: dc01.aether.test (localhost)"
echo "[+] KDC: localhost:88"
echo "[+] LDAP: localhost:389"
echo ""
echo "Next step: Run seed script: ./scripts/ad-lab/seed.sh"