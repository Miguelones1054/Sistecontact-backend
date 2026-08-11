#!/usr/bin/env bash
# Ejecutar desde tu Mac para desplegar en el VPS.
# Uso:
#   bash deploy/remote-deploy.sh
#   bash deploy/remote-deploy.sh root@167.99.149.19

set -euo pipefail

HOST="${1:-root@167.99.149.19}"
REMOTE_SCRIPT="/root/Sistecontact-api/deploy/deploy.sh"

echo "==> Deploy remoto en $HOST"
ssh "$HOST" "bash $REMOTE_SCRIPT"
