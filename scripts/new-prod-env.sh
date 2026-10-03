#!/usr/bin/env bash
# Generate a production .env.prod from .env.prod.example with a random
# Postgres password wired into both POSTGRES_PASSWORD and DATABASE_URL.
#
# Usage:
#   ./scripts/new-prod-env.sh http://YOUR_VPS_IP:5000 http://YOUR_VPS_IP:3000
#
# The two URLs become PUBLIC_API_URL and CORS_ALLOWED_ORIGINS. Re-run to
# rotate the password (then `docker compose ... up -d` to apply).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEMPLATE="$ROOT/.env.prod.example"
TARGET="$ROOT/.env.prod"

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <PUBLIC_API_URL> <PUBLIC_DASHBOARD_URL>" >&2
  echo "example: $0 http://203.0.113.10:5000 http://203.0.113.10:3000" >&2
  exit 1
fi

if ! command -v openssl >/dev/null 2>&1; then
  echo "error: openssl is required to generate the password" >&2
  exit 1
fi

PUBLIC_API_URL="$1"
PUBLIC_DASHBOARD_URL="$2"
PASSWORD="$(openssl rand -hex 32)"

sed \
  -e "s|CHANGE_ME__run_scripts_new_prod_env_sh|$PASSWORD|g" \
  -e "s|http://YOUR_VPS_IP:5000|$PUBLIC_API_URL|g" \
  -e "s|http://YOUR_VPS_IP:3000|$PUBLIC_DASHBOARD_URL|g" \
  "$TEMPLATE" > "$TARGET"
chmod 600 "$TARGET"

cat <<EOF
wrote $TARGET (mode 600)
  PUBLIC_API_URL=$PUBLIC_API_URL
  CORS_ALLOWED_ORIGINS=$PUBLIC_DASHBOARD_URL
next:
  1. review $TARGET
  2. docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml up -d --build
EOF
