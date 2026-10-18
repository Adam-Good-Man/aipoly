#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ART="$ROOT/deploy/artifacts"

echo "==> Building API (linux/amd64)..."
mkdir -p "$ART"
cd "$ROOT/api"
env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 GOSUMDB=sum.golang.org GOTOOLCHAIN=auto \
  go build -ldflags="-s -w" -o "$ART/aipoly" .

echo "==> Building client..."
cd "$ROOT/client"
npm ci --silent 2>/dev/null || npm install --silent
npm run build
rm -rf "$ART/client-dist"
cp -R dist "$ART/client-dist"

echo "==> Artifacts ready in deploy/artifacts/"
