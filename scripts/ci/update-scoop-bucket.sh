#!/usr/bin/env bash
# Bump the umbra manifest in the bucket. Absent token is a skip rather than a
# failure, matching the Homebrew step.
set -euo pipefail

if [ -z "${SCOOP_WRITE_TOKEN:-}" ]; then
  echo "::warning::SCOOP_WRITE_TOKEN is absent; skipping Scoop update" >&2
  exit 0
fi

# Clone dir is named for the repo, not "bucket": the manifest lives in the
# repo's own bucket/ dir, and two nested "bucket" paths read as a typo.
git clone --depth 1 \
  https://forgejo.coilysiren.me/coilyco-flight-deck/scoop-bucket.git scoop-bucket
cp dist/umbra.json scoop-bucket/bucket/umbra.json
cd scoop-bucket
git add bucket/umbra.json
if git diff --cached --quiet; then
  exit 0
fi

git config user.name "coilyco-ops"
git config user.email "coilyco-ops@coilysiren.me"
git commit -m "chore(umbra): bump manifest to ${TAG} [skip ci]"
git push \
  "https://coilyco-ops:${SCOOP_WRITE_TOKEN}@forgejo.coilysiren.me/coilyco-flight-deck/scoop-bucket.git" \
  HEAD:main
