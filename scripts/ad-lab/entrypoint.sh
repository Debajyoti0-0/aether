#!/bin/bash
# Stage 45b — Samba AD DC Entrypoint

set -euo pipefail

echo "[+] Starting Samba AD DC Entrypoint..."

# Run provisioning
if [ -f /provision.sh ]; then
    echo "[+] Running provisioning script..."
    /provision.sh || echo "[!] Provisioning had issues, continuing..."
fi

# Start Samba
echo "[+] Starting Samba AD DC..."
exec /usr/sbin/samba --foreground --no-process-group