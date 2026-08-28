#!/usr/bin/env bash
# Point Go's caches and temp dirs at the runner's scratch space, which is large
# and per-job, instead of the container's small default /tmp.
set -euo pipefail

scratch="${RUNNER_TEMP:-/tmp}"
mkdir -p \
  "$scratch/go-tmp" \
  "$scratch/go-build" \
  "$scratch/go-mod" \
  "$scratch/xdg-cache"
{
  echo "GOTMPDIR=$scratch/go-tmp"
  echo "TMPDIR=$scratch/go-tmp"
  echo "GOCACHE=$scratch/go-build"
  echo "GOMODCACHE=$scratch/go-mod"
  echo "XDG_CACHE_HOME=$scratch/xdg-cache"
} >> "$GITHUB_ENV"
