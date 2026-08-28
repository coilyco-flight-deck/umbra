#!/usr/bin/env bash
# Filesystem secret scan. The exclude list is one throwaway input to one run, so
# it is a temp file with a cleanup trap rather than the fixed path used inline.
set -euo pipefail

exclude="$(mktemp)"
trap 'rm -f "$exclude"' EXIT
printf '(^|/)\.git/\n' > "$exclude"

trufflehog filesystem . \
  --no-verification --no-update --fail \
  --exclude-paths="$exclude"
