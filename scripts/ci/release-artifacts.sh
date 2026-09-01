#!/usr/bin/env bash
# Build, package, and verify the umbra release artifacts for one version.
set -euo pipefail

make release-artifacts VERSION="${VERSION}" DIST_DIR=dist
make release-package VERSION="${VERSION}" DIST_DIR=dist
make release-check VERSION="${VERSION}" DIST_DIR=dist
