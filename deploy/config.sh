#!/usr/bin/env bash
# Shared deploy settings — override via environment if needed.
REMOTE_HOST="${REMOTE_HOST:-root@dornavel.com}"
REMOTE_DIR="${REMOTE_DIR:-/opt/aipoly}"
API_PORT="${API_PORT:-8081}"
WEB_PORT="${WEB_PORT:-8091}"
DOMAIN="${DOMAIN:-aipoly.dornavel.com}"
AIPOLY_DB_NAME="${AIPOLY_DB_NAME:-aipoly}"
AIPOLY_DB_USER="${AIPOLY_DB_USER:-dornavel}"
AIPOLY_DB_PASSWORD="${AIPOLY_DB_PASSWORD:-Kuloutou@184}"
