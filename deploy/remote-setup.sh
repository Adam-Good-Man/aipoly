#!/usr/bin/env bash
# Runs ON THE SERVER after files are synced to /opt/aipoly
set -euo pipefail

REMOTE_DIR="${REMOTE_DIR:-/opt/aipoly}"
API_PORT="${API_PORT:-8081}"
WEB_PORT="${WEB_PORT:-8091}"
DB_NAME="${DB_NAME:-aipoly}"
DB_USER="${DB_USER:-aipoly}"
DB_PASSWORD="${DB_PASSWORD:?DB_PASSWORD required}"

echo "==> PostgreSQL: ensure database ${DB_NAME}..."
sudo -u postgres psql -tc "SELECT 1 FROM pg_roles WHERE rolname='${DB_USER}'" | grep -q 1 || \
  sudo -u postgres psql -c "CREATE USER ${DB_USER} WITH PASSWORD '${DB_PASSWORD}';"
sudo -u postgres psql -tc "SELECT 1 FROM pg_database WHERE datname='${DB_NAME}'" | grep -q 1 || \
  sudo -u postgres psql -c "CREATE DATABASE ${DB_NAME} OWNER ${DB_USER};"
sudo -u postgres psql -c "GRANT ALL PRIVILEGES ON DATABASE ${DB_NAME} TO ${DB_USER};" 2>/dev/null || true
sudo -u postgres psql -d "${DB_NAME}" -c "GRANT ALL ON SCHEMA public TO ${DB_USER};" 2>/dev/null || true
sudo -u postgres psql -d "${DB_NAME}" -c "ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO ${DB_USER};" 2>/dev/null || true

echo "==> Permissions..."
chmod +x "${REMOTE_DIR}/api/aipoly"
chmod 600 "${REMOTE_DIR}/api/.env"

echo "==> systemd..."
install -m 0644 "${REMOTE_DIR}/deploy/systemd/aipoly-api.service" /etc/systemd/system/aipoly-api.service
systemctl daemon-reload
systemctl enable aipoly-api.service
systemctl restart aipoly-api.service

echo "==> nginx..."
install -m 0644 "${REMOTE_DIR}/deploy/nginx/aipoly-ip.conf" /etc/nginx/conf.d/aipoly-ip.conf
rm -f /etc/nginx/conf.d/aipoly.conf

if [[ -f "/etc/letsencrypt/live/aipoly.dornavel.com/fullchain.pem" ]]; then
  install -m 0644 "${REMOTE_DIR}/deploy/nginx/aipoly.dornavel.com.conf" /etc/nginx/conf.d/aipoly.dornavel.com.conf
else
  install -m 0644 "${REMOTE_DIR}/deploy/nginx/aipoly.dornavel.com.http.conf" /etc/nginx/conf.d/aipoly.dornavel.com.conf
fi

nginx -t
systemctl reload nginx

echo "==> firewall (port ${WEB_PORT})..."
if command -v firewall-cmd >/dev/null 2>&1; then
  firewall-cmd --permanent --add-port="${WEB_PORT}/tcp" 2>/dev/null || true
  firewall-cmd --reload 2>/dev/null || true
fi

sleep 2
if systemctl is-active --quiet aipoly-api; then
  echo "==> aipoly-api: running"
else
  echo "==> aipoly-api: FAILED — journalctl -u aipoly-api -n 50"
  journalctl -u aipoly-api -n 30 --no-pager || true
  exit 1
fi

curl -sf "http://127.0.0.1:${API_PORT}/api/dashboard" >/dev/null && echo "==> API health: OK" || echo "==> API health: check logs"
