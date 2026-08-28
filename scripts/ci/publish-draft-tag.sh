#!/usr/bin/env bash
# Publish the draft tag that release.yml later verifies, proving the promoted
# commit is the one CI gated.
set -euo pipefail

if [ -z "${PROMOTE_TOKEN:-}" ]; then
  echo "::error::CI_RELEASE_TOKEN not set; draft release publishing needs a real-user PAT so the push is attributable" >&2
  exit 1
fi

# server_url resolves to the in-cluster http URL on this runner, so keep its
# scheme instead of assuming https.
proto="${SERVER%%://*}"
host="${SERVER#*://}"
remote_url="${proto}://oauth2:${PROMOTE_TOKEN}@${host}/${REPO}.git"
draft_tag="draft-${GITHUB_SHA}"

if git ls-remote --exit-code --tags "${remote_url}" "refs/tags/${draft_tag}" >/dev/null 2>&1; then
  echo "draft tag already published: ${draft_tag}"
  exit 0
fi

git tag "${draft_tag}"
git push "${remote_url}" "refs/tags/${draft_tag}"
echo "published ${draft_tag}"
