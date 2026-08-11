#!/usr/bin/env bash
# Despliegue en el VPS: pull + build + reinicio del servicio.
# Uso (en el servidor):
#   cd /root/Sistecontact-api && bash deploy/deploy.sh
# O desde tu Mac:
#   ssh root@167.99.149.19 'bash /root/Sistecontact-api/deploy/deploy.sh'

set -euo pipefail

APP_DIR="/root/Sistecontact-api"
SERVICE="sistecontact-api"
BRANCH="${DEPLOY_BRANCH:-main}"

cd "$APP_DIR"

echo "==> [$(date -Iseconds)] Deploy SisteContact API"
echo "==> Rama: $BRANCH"

if [[ ! -f .env ]]; then
  echo "ERROR: falta $APP_DIR/.env" >&2
  exit 1
fi

if [[ ! -f sistecontact-firebase-adminsdk-fbsvc-8adb9b6483.json ]] \
  && [[ -z "${FIREBASE_CREDENTIALS_FILE:-}" ]]; then
  # Intenta el nombre por defecto del .env
  CRED="$(grep -E '^FIREBASE_CREDENTIALS_FILE=' .env 2>/dev/null | cut -d= -f2- || true)"
  if [[ -n "${CRED}" && ! -f "$CRED" ]]; then
    echo "ERROR: falta el JSON de Firebase Admin: $CRED" >&2
    exit 1
  fi
fi

echo "==> git fetch / reset"
git fetch --all --prune
git checkout "$BRANCH"
git reset --hard "origin/$BRANCH"

echo "==> go build"
export PATH="/usr/local/go/bin:${PATH}"
mkdir -p bin
go build -o bin/sistecontact-api ./cmd/server

echo "==> reiniciar $SERVICE"
systemctl restart "$SERVICE"
systemctl --no-pager --full status "$SERVICE" | head -20

echo "==> health check"
sleep 1
curl -fsS "http://127.0.0.1:8091/api/health" || true
echo
echo "==> Deploy OK"
