#!/usr/bin/env bash
# Build the tracker bundle and stage it for the API Docker image.
#
# The API image COPYs services/api/tracker-dist, but that directory is
# gitignored (only .gitkeep is committed). Building the image without
# running this first bakes an empty directory and /analytics.js 404s.
#
# Uses a disposable Node container when pnpm is not on the host, so a
# minimal VPS needs nothing installed. The esbuild bundle uses tens of
# MB — safe on 1 GiB boxes. Run from anywhere:
#   ./scripts/build-tracker-dist.sh
set -euo pipefail

# Never prompt for pnpm downloads: this script must run unattended.
export COREPACK_ENABLE_DOWNLOAD_PROMPT=0

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TARGET="$ROOT/services/api/tracker-dist"

# Root-owned residue (e.g. packages/tracker/dist from an earlier
# docker-as-root build) makes esbuild fail with "permission denied".
# Repair ownership of the trees this script writes to.
fix_owner() {
  [ -e "$1" ] || return 0
  [ -w "$1" ] && return 0
  echo "fixing ownership of $1 (asks for sudo once)..."
  sudo chown -R "$(id -u):$(id -g)" "$1"
}
fix_owner "$ROOT/packages/tracker"
fix_owner "$ROOT/node_modules"
fix_owner "$TARGET"

build_host() {
  pnpm install --frozen-lockfile
  pnpm --filter @devpulse/tracker build
}

build_container() {
  # Same uid as the caller so node_modules/dist stay owned by them.
  # Writable HOME/cache inside the container (the image default is root's).
  docker run --rm \
    --user "$(id -u):$(id -g)" \
    -v "$ROOT:/work" -w /work \
    -e HOME=/tmp -e COREPACK_HOME=/tmp/corepack \
    -e NPM_CONFIG_CACHE=/tmp/npm-cache \
    node:22-alpine sh -c "corepack enable && corepack prepare pnpm@10.34.5 --activate && pnpm install --frozen-lockfile --store-dir /tmp/pnpm-store && pnpm --filter @devpulse/tracker build"
}

if command -v pnpm >/dev/null 2>&1; then
  echo "building tracker with host pnpm..."
  (cd "$ROOT" && build_host)
else
  echo "no host pnpm; building tracker in a disposable node:22 container..."
  if ! command -v docker >/dev/null 2>&1; then
    echo "error: need pnpm or docker to build the tracker bundle" >&2
    exit 1
  fi
  build_container
fi

rm -f "$TARGET"/analytics*.js
cp "$ROOT"/packages/tracker/dist/analytics*.js "$TARGET/"
echo "staged:"
ls -la "$TARGET"
