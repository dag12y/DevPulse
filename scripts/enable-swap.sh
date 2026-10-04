#!/usr/bin/env bash
# Add a swap file so a 1 GiB VPS survives memory spikes (apt, docker pull,
# Go builds) instead of freezing / OOM-killing Postgres.
#
# Usage: sudo ./scripts/enable-swap.sh [size_GB, default 2]
set -euo pipefail

SIZE_GB="${1:-2}"
SWAPFILE="/swapfile"

if swapon --show | grep -q "$SWAPFILE"; then
  echo "swap already active on $SWAPFILE"
  swapon --show
  exit 0
fi

# Repair a leftover /swapfile with wrong perms or no swap header
# (e.g. empty file from a failed run): enforce size + perms + signature.
EXPECTED_BYTES=$(( SIZE_GB * 1024 * 1024 * 1024 ))
if [[ -f "$SWAPFILE" ]]; then
  ACTUAL_BYTES="$(stat -c%s "$SWAPFILE" 2>/dev/null || echo 0)"
  if [[ "$ACTUAL_BYTES" -ne "$EXPECTED_BYTES" ]]; then
    echo "recreating $SWAPFILE (${ACTUAL_BYTES}B -> ${EXPECTED_BYTES}B)..."
    swapoff "$SWAPFILE" 2>/dev/null || true
    rm -f "$SWAPFILE"
  fi
fi

if [[ ! -f "$SWAPFILE" ]]; then
  echo "creating ${SIZE_GB}G swapfile at $SWAPFILE..."
  fallocate -l "${SIZE_GB}G" "$SWAPFILE" || dd if=/dev/zero of="$SWAPFILE" bs=1M count=$(( SIZE_GB * 1024 )) status=progress
fi

chmod 600 "$SWAPFILE"
if ! blkid -t TYPE=swap "$SWAPFILE" >/dev/null 2>&1; then
  echo "formatting $SWAPFILE as swap..."
  mkswap "$SWAPFILE"
fi

swapon "$SWAPFILE"
grep -q "$SWAPFILE" /etc/fstab || echo "$SWAPFILE none swap sw 0 0" >> /etc/fstab

# Prefer keeping app pages in RAM, but allow swap before the OOM killer.
sysctl -w vm.swappiness=60 >/dev/null
sysctl -w vm.vfs_cache_pressure=100 >/dev/null

echo "--- memory ---"
free -h
swapon --show
