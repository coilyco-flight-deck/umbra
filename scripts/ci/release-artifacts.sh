#!/usr/bin/env bash
# Build, package, and verify the umbra release artifacts for one version.
set -euo pipefail

./scripts/build-umbra-release.sh "${VERSION}" dist
./scripts/render-umbra-packaging.sh "${VERSION}" dist
./scripts/check-umbra-release.sh "${VERSION}" dist
