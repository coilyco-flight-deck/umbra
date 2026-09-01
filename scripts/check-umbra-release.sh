#!/bin/sh
# Verify checksums, package metadata, and the native umbra release binary.
set -eu

version=${1:?usage: check-umbra-release.sh VERSION [DIST_DIR]}
dist=${2:-dist}
bare=${version#v}

case "$version" in
  v[0-9]*.[0-9]*.[0-9]*) ;;
  *)
    echo "umbra release version must be a v-prefixed semantic version, got: $version" >&2
    exit 2
    ;;
esac

(
  cd "$dist"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum -c SHA256SUMS
  else
    shasum -a 256 -c SHA256SUMS
  fi
)

checksum_count=$(wc -l < "$dist/SHA256SUMS" | tr -d ' ')
if [ "$checksum_count" -ne 6 ]; then
  echo "umbra release must contain six checksummed binaries" >&2
  exit 1
fi

python3 -m json.tool "$dist/umbra.json" >/dev/null
if command -v ruby >/dev/null 2>&1; then
  ruby -c "$dist/umbra.rb" >/dev/null
fi

grep -F "version \"${bare}\"" "$dist/umbra.rb" >/dev/null
grep -F "\"version\": \"${bare}\"" "$dist/umbra.json" >/dev/null

for artifact in \
  umbra-darwin-amd64 \
  umbra-darwin-arm64 \
  umbra-linux-amd64 \
  umbra-linux-arm64
do
  grep -F "$artifact" "$dist/umbra.rb" >/dev/null
done

for artifact in umbra-windows-amd64.exe umbra-windows-arm64.exe
do
  grep -F "$artifact" "$dist/umbra.json" >/dev/null
done

case "$(uname -s)/$(uname -m)" in
  Darwin/x86_64) native="$dist/umbra-darwin-amd64" ;;
  Darwin/arm64) native="$dist/umbra-darwin-arm64" ;;
  Linux/x86_64) native="$dist/umbra-linux-amd64" ;;
  Linux/aarch64 | Linux/arm64) native="$dist/umbra-linux-arm64" ;;
  *) native="" ;;
esac

if [ -n "$native" ]; then
  actual=$("$native" --version)
  expected="umbra version ${version} (umbra ref ${version})"
  if [ "$actual" != "$expected" ]; then
    echo "native binary reports $actual, expected $expected" >&2
    exit 1
  fi
fi

echo "verified umbra release $version"
