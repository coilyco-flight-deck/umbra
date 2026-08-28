#!/usr/bin/env bash
# Fast-forward main onto the release branch.
set -euo pipefail

if [ -z "${PROMOTE_TOKEN:-}" ]; then
  echo "::error::CI_RELEASE_TOKEN not set; promotion needs a real-user PAT so the release push is attributable" >&2
  exit 1
fi

# server_url resolves to the in-cluster http URL on this runner, so keep its
# scheme instead of assuming https.
proto="${SERVER%%://*}"
host="${SERVER#*://}"

# Plain push: git refuses a non-fast-forward, so a diverged release branch fails
# loud here instead of being silently rewritten.
git push "${proto}://oauth2:${PROMOTE_TOKEN}@${host}/${REPO}.git" HEAD:release
echo "promoted $(git rev-parse --short HEAD) to release"
