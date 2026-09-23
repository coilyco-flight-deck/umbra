# Per-repo task manifest. Run `just` (or `just --list`) to see every verb.
#
# One line of comment per recipe on purpose: just reads only the LAST comment
# line above a recipe, so a wrapped description silently truncates to its tail.

set positional-arguments

# Default target: list every available recipe.
default:
    @just --list --unsorted

# Build all packages.
build *ARGS:
    go build ./... "$@"

# Run the unit test suite.
test *ARGS:
    go test ./... "$@"

# go vet across the tree.
vet *ARGS:
    go vet ./... "$@"

# Lint with golangci-lint.
lint *ARGS:
    golangci-lint run ./... "$@"

# go mod tidy.
tidy:
    go mod tidy

# Format Go source.
fmt:
    gofmt -w $(find . -name '*.go' -not -path './vendor/*')

# Unit tests with a coverage profile.
cover *ARGS:
    go test -coverprofile=coverage.out ./... "$@"

# Render the CLI reference under site/cli/ via cli-web-docs.
docs-cli:
    cd scripts/gen-webdocs && go run .

# Regenerate godoc-current.txt, then commit the diff to land API changes.
godoc-update:
    ./scripts/check-godoc-current.sh --update

# Build the tagged umbra binary matrix and SHA256SUMS.
release-artifacts VERSION DIST_DIR="dist":
    ./scripts/build-umbra-release.sh "$1" "$2"

# Render Homebrew and Scoop metadata from tagged umbra binaries.
release-package VERSION DIST_DIR="dist":
    ./scripts/render-umbra-packaging.sh "$1" "$2"

# Verify umbra checksums, package metadata, and native version.
release-check VERSION DIST_DIR="dist":
    ./scripts/check-umbra-release.sh "$1" "$2"

# Run every repository hook.
pre-commit *ARGS:
    pre-commit run --all-files "$@"
