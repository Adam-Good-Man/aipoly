#!/usr/bin/env bash
# Sync local aipoly PostgreSQL → remote server (handles column-order differences)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"

LOCAL_HOST="${LOCAL_DB_HOST:-localhost}"
LOCAL_PORT="${LOCAL_DB_PORT:-5432}"
LOCAL_USER="${LOCAL_DB_USER:-postgres}"
LOCAL_DB="${LOCAL_DB_NAME:-aipoly}"
LOCAL_PASS="${LOCAL_DB_PASSWORD:-$(grep '^DB_PASSWORD=' "$ROOT/.env" | cut -d= -f2-)}"

REMOTE_HOST="${REMOTE_DB_HOST:-203.161.62.155}"
REMOTE_PORT="${REMOTE_DB_PORT:-5432}"
REMOTE_USER="${REMOTE_DB_USER:-dornavel}"
REMOTE_DB="${REMOTE_DB_NAME:-aipoly}"
REMOTE_PASS="${REMOTE_DB_PASSWORD:-Kuloutou@184}"

WORKDIR=$(mktemp -d)
trap 'rm -rf "$WORKDIR"' EXIT

echo "==> Export local ${LOCAL_DB}..."
export PGPASSWORD="$LOCAL_PASS"
psql -h "$LOCAL_HOST" -p "$LOCAL_PORT" -U "$LOCAL_USER" -d "$LOCAL_DB" -v ON_ERROR_STOP=1 \
  -c "\\copy portfolios TO '$WORKDIR/portfolios.csv' WITH (FORMAT csv, HEADER true)"
psql -h "$LOCAL_HOST" -p "$LOCAL_PORT" -U "$LOCAL_USER" -d "$LOCAL_DB" -v ON_ERROR_STOP=1 \
  -c "\\copy (SELECT id, market_id, condition_id, event_title, market_title, outcome, side, amount, price, shares, reasoning, self_win_rate, status, pnl, is_paper, token_id, clob_order_id, order_error, created_at, resolved_at FROM trades) TO '$WORKDIR/trades.csv' WITH (FORMAT csv, HEADER true)"
psql -h "$LOCAL_HOST" -p "$LOCAL_PORT" -U "$LOCAL_USER" -d "$LOCAL_DB" -v ON_ERROR_STOP=1 \
  -c "\\copy analysis_logs TO '$WORKDIR/analysis_logs.csv' WITH (FORMAT csv, HEADER true)"
psql -h "$LOCAL_HOST" -p "$LOCAL_PORT" -U "$LOCAL_USER" -d "$LOCAL_DB" -v ON_ERROR_STOP=1 \
  -c "\\copy market_events TO '$WORKDIR/market_events.csv' WITH (FORMAT csv, HEADER true)"

echo "==> Import to remote ${REMOTE_HOST}/${REMOTE_DB}..."
export PGPASSWORD="$REMOTE_PASS"
psql -h "$REMOTE_HOST" -p "$REMOTE_PORT" -U "$REMOTE_USER" -d "$REMOTE_DB" -v ON_ERROR_STOP=1 \
  -c "TRUNCATE portfolios, trades, analysis_logs, market_events;"
psql -h "$REMOTE_HOST" -p "$REMOTE_PORT" -U "$REMOTE_USER" -d "$REMOTE_DB" -v ON_ERROR_STOP=1 \
  -c "\\copy portfolios FROM '$WORKDIR/portfolios.csv' WITH (FORMAT csv, HEADER true)"
psql -h "$REMOTE_HOST" -p "$REMOTE_PORT" -U "$REMOTE_USER" -d "$REMOTE_DB" -v ON_ERROR_STOP=1 \
  -c "\\copy trades FROM '$WORKDIR/trades.csv' WITH (FORMAT csv, HEADER true)"
psql -h "$REMOTE_HOST" -p "$REMOTE_PORT" -U "$REMOTE_USER" -d "$REMOTE_DB" -v ON_ERROR_STOP=1 \
  -c "\\copy analysis_logs FROM '$WORKDIR/analysis_logs.csv' WITH (FORMAT csv, HEADER true)"
psql -h "$REMOTE_HOST" -p "$REMOTE_PORT" -U "$REMOTE_USER" -d "$REMOTE_DB" -v ON_ERROR_STOP=1 \
  -c "\\copy market_events FROM '$WORKDIR/market_events.csv' WITH (FORMAT csv, HEADER true)"

echo "==> Remote row counts:"
psql -h "$REMOTE_HOST" -p "$REMOTE_PORT" -U "$REMOTE_USER" -d "$REMOTE_DB" -c \
  "SELECT 'trades' t, count(*) FROM trades UNION ALL SELECT 'portfolios', count(*) FROM portfolios UNION ALL SELECT 'analysis_logs', count(*) FROM analysis_logs UNION ALL SELECT 'market_events', count(*) FROM market_events;"
echo "==> Done."
