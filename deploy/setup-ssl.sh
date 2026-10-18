#!/usr/bin/env bash
# 在服务器上运行：申请 aipoly.dornavel.com 证书并切换到 HTTPS 配置
set -euo pipefail

REMOTE_DIR="${REMOTE_DIR:-/opt/aipoly}"
DOMAIN="${DOMAIN:-aipoly.dornavel.com}"
CERTBOT_EMAIL="${CERTBOT_EMAIL:-admin@dornavel.com}"
NGINX_CONF="/etc/nginx/conf.d/aipoly.dornavel.com.conf"
CERT_PATH="/etc/letsencrypt/live/${DOMAIN}/fullchain.pem"

mkdir -p /var/www/certbot

install_nginx() {
  local src="$1"
  install -m 0644 "${REMOTE_DIR}/deploy/nginx/${src}" "${NGINX_CONF}"
  rm -f /etc/nginx/conf.d/aipoly.conf
  nginx -t
  systemctl reload nginx
}

if [[ -f "$CERT_PATH" ]]; then
  echo "==> Certificate already exists, enabling HTTPS config..."
  install_nginx "aipoly.dornavel.com.conf"
  echo "==> HTTPS ready: https://${DOMAIN}"
  exit 0
fi

echo "==> Step 1: HTTP-only config (for ACME)..."
install_nginx "aipoly.dornavel.com.http.conf"

echo "==> Step 2: certbot webroot..."
certbot certonly --webroot \
  -w /var/www/certbot \
  -d "${DOMAIN}" \
  --non-interactive \
  --agree-tos \
  -m "${CERTBOT_EMAIL}" \
  --no-eff-email

echo "==> Step 3: Enable HTTPS config..."
install_nginx "aipoly.dornavel.com.conf"

echo "==> Done: https://${DOMAIN}"
