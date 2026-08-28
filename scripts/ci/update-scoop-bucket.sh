#!/usr/bin/env bash
# Bump the specgen manifest in the bucket. Absent token is a skip rather than a
# failure, matching the Homebrew step.
set -euo pipefail

if [ -z "${SCOOP_WRITE_TOKEN:-}" ]; then
  echo "::warning::SCOOP_WRITE_TOKEN is absent; skipping Scoop update" >&2
  exit 0
fi

git clone --depth 1 \
  https://forgejo.coilysiren.me/coilyco-flight-deck/scoop-bucket.git bucket
cp dist/specgen.json bucket/bucket/specgen.json
cd bucket
git add bucket/specgen.json
if git diff --cached --quiet; then
  exit 0
fi

git config user.name "coilyco-ops"
git config user.email "coilyco-ops@coilysiren.me"
git commit -m "chore(specgen): bump manifest to ${TAG} [skip ci]"
git push \
  "https://coilyco-ops:${SCOOP_WRITE_TOKEN}@forgejo.coilysiren.me/coilyco-flight-deck/scoop-bucket.git" \
  HEAD:main
