#!/usr/bin/env bash
# Bump the umbra formula in the tap. Absent token is a skip rather than a
# failure, so a release without tap credentials still completes.
set -euo pipefail

if [ -z "${TAP_WRITE_TOKEN:-}" ]; then
  echo "::warning::TAP_WRITE_TOKEN is absent; skipping Homebrew update" >&2
  exit 0
fi

git clone --depth 1 \
  https://forgejo.coilysiren.me/coilyco-flight-deck/homebrew-tap.git tap
cp dist/umbra.rb tap/Formula/umbra.rb
cd tap
git add Formula/umbra.rb
if git diff --cached --quiet; then
  exit 0
fi

git config user.name "coilyco-ops"
git config user.email "coilyco-ops@coilysiren.me"
git commit -m "chore(umbra): bump formula to ${TAG} [skip ci]"
git push \
  "https://coilyco-ops:${TAP_WRITE_TOKEN}@forgejo.coilysiren.me/coilyco-flight-deck/homebrew-tap.git" \
  HEAD:main
