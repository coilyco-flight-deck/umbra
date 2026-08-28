#!/usr/bin/env bash
# Tidy must be a no-op in CI: a dirty go.mod or go.sum means the commit did not
# run it.
set -euo pipefail

go mod tidy
git diff --exit-code -- go.mod go.sum
