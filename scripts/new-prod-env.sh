#!/usr/bin/env bash
# Generate a production .env.prod from .env.prod.example with a random
# Postgres password wired into both POSTGRES_PASSWORD and DATABASE_URL.
#
# Usage:
#   ./scripts/new-prod-env.sh http://YOUR_VPS_IP:5000 https://YOUR_APP.vercel.app
#
# The two URLs become PUBLIC_API_URL and CORS_ALLOWED_ORIGINS. Re-run to
# rotate the password (then `docker compose ... up -d` to apply).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TEMPLATE="$ROOT/.env.prod.example"
TARGET="$ROOT/.env.prod"

if [[ $# -ne 2 ]]; then
  echo "usage: $0 <PUBLIC_API_URL> <PUBLIC_DASHBOARD_URL>" >&2
  echo "example: $0 http://203.0.113.10:5000 https://my-app.vercel.app" >&2
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
  -e "s|https://YOUR_APP.vercel.app|$PUBLIC_DASHBOARD_URL|g" \
  "$TEMPLATE" > "$TARGET"
chmod 600 "$TARGET"

cat <<EOF
wrote $TARGET (mode 600)
  PUBLIC_API_URL=$PUBLIC_API_URL
  CORS_ALLOWED_ORIGINS=$PUBLIC_DASHBOARD_URL
next (1 GiB VPS: dashboard on Vercel, API + Postgres on VPS):
  1. review $TARGET (set CORS_ALLOWED_ORIGINS to your Vercel URL)
  2. set RESEND_API_KEY and EMAIL_FROM (auth email; the API refuses to
     boot in production without them) and check APP_URL
  3. on the VPS: sudo ./scripts/enable-swap.sh 2
  4. docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.vps.yml up -d --build
  5. deploy apps/dashboard on Vercel with NEXT_PUBLIC_API_URL=$PUBLIC_API_URL
full-stack alternative (2GB+ RAM only):
  docker compose --env-file .env.prod -f docker-compose.yml -f docker-compose.prod.yml up -d --build
EOF
