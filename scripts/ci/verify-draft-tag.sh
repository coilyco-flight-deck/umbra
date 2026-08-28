#!/usr/bin/env bash
# The draft tag must already exist on main, so a release can only be cut from a
# commit that passed the promote gate.
set -euo pipefail

git fetch --tags --force
git rev-parse --verify "refs/tags/${DRAFT_TAG}" >/dev/null
echo "verified ${DRAFT_TAG}"
