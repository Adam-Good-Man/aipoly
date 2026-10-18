#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
source deploy/config.sh

ENV_FILE="${ENV_FILE:-$ROOT/.env}"
if [[ ! -f "$ENV_FILE" ]]; then
  echo "Missing $ENV_FILE — copy .env.example and configure secrets."
  exit 1
fi

DB_PASSWORD="$(grep '^DB_PASSWORD=' "$ENV_FILE" | cut -d= -f2- | tr -d '"')"
REMOTE_DB_PASSWORD="${AIPOLY_DB_PASSWORD:-Kuloutou@184}"

echo "==> Build..."
bash deploy/build.sh

ART="$ROOT/deploy/artifacts"
TMP_ENV="$(mktemp)"
# Production .env: local DB, no China proxy on VPS
grep -v '^HTTP_PROXY=' "$ENV_FILE" | grep -v '^HTTPS_PROXY=' | \
  sed 's/^DB_HOST=.*/DB_HOST=127.0.0.1/' | \
  sed "s/^DB_USER=.*/DB_USER=${AIPOLY_DB_USER}/" | \
  sed "s/^DB_NAME=.*/DB_NAME=${AIPOLY_DB_NAME}/" | \
  sed "s/^DB_PASSWORD=.*/DB_PASSWORD=${REMOTE_DB_PASSWORD}/" > "$TMP_ENV"
echo "API_PORT=${API_PORT}" >> "$TMP_ENV"

echo "==> Sync to ${REMOTE_HOST}:${REMOTE_DIR} ..."
ssh "$REMOTE_HOST" "mkdir -p ${REMOTE_DIR}/api ${REMOTE_DIR}/client/dist ${REMOTE_DIR}/deploy/systemd ${REMOTE_DIR}/deploy/nginx"

rsync -az --delete "$ART/aipoly" "${REMOTE_HOST}:${REMOTE_DIR}/api/aipoly"
rsync -az --delete "$ART/client-dist/" "${REMOTE_HOST}:${REMOTE_DIR}/client/dist/"
rsync -az "$TMP_ENV" "${REMOTE_HOST}:${REMOTE_DIR}/api/.env"
rsync -az deploy/nginx/ "${REMOTE_HOST}:${REMOTE_DIR}/deploy/nginx/"
rsync -az deploy/setup-ssl.sh "${REMOTE_HOST}:${REMOTE_DIR}/deploy/setup-ssl.sh"
rsync -az deploy/systemd/aipoly-api.service "${REMOTE_HOST}:${REMOTE_DIR}/deploy/systemd/"
rsync -az deploy/remote-setup.sh "${REMOTE_HOST}:${REMOTE_DIR}/deploy/remote-setup.sh"

rm -f "$TMP_ENV"

echo "==> Remote setup..."
ssh "$REMOTE_HOST" "chmod +x ${REMOTE_DIR}/deploy/remote-setup.sh && \
  REMOTE_DIR=${REMOTE_DIR} API_PORT=${API_PORT} WEB_PORT=${WEB_PORT} \
  DB_NAME=${AIPOLY_DB_NAME} DB_USER=${AIPOLY_DB_USER} DB_PASSWORD='${REMOTE_DB_PASSWORD}' \
  bash ${REMOTE_DIR}/deploy/remote-setup.sh"

SERVER_IP="$(ssh "$REMOTE_HOST" "hostname -I | awk '{print \$1}'")"

echo ""
echo "=========================================="
echo "  aiPoly deployed successfully"
echo "=========================================="
echo "  Web (port ${WEB_PORT}):  http://${SERVER_IP}:${WEB_PORT}"
echo "  Web (domain):            http://${DOMAIN}  (需 DNS A 记录 → ${SERVER_IP})"
echo "  API (internal):          127.0.0.1:${API_PORT}"
echo "=========================================="
