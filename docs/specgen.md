# the no-code driver (specgen / cmd/specgen)

`ward-kdl` is a **no-code** CLI: the consumer authors policy plus committed locks, never Go or build glue. `specgen` is its driver. Every spec may carry a top-level [`description`](value-providers.md) node for standing context.

## Install

Homebrew (`brew install coilyco-flight-deck/tap/specgen`) and Scoop (`scoop install coilyco/specgen`) track the coilyco tap and bucket. Each umbra tag also publishes raw binaries for Linux, macOS, and Windows on amd64 and arm64, verified against the release `SHA256SUMS`. Go users run `GOPRIVATE=forgejo.coilysiren.me go install .../cmd/specgen@vX.Y.Z`.

## Discovery and merging

A `--guardfile` selects a **binary**, not the whole build: members compose only when their parsed `wrap <binary>` name (`Group[0]`) agrees, and a different wrap name is a separate binary never merged in. With no flags, a `.specgen/` directory in the cwd is the recursive project boundary; `--project-root <dir>` selects another. Every `.kdl` below the root is inspected and a member is recognized by a top-level `wrap` declaration rather than its filename. With no selector, exactly one binary group must be present; more fails with a sorted actionable list.

Member paths are normalized relative to the root and sorted lexically before rendering, hashing, locking, or building, so re-rooting an unchanged project preserves generated order and cache identity. Per-member locks retain those directories, so identically named members in separate folders cannot overwrite one another. Parsed KDL without a top-level `wrap` is unrelated configuration and ignored, but a malformed file declaring `wrap` is not. Unreadable candidates, duplicate members, conflicting artifacts, and symlinks escaping the root all fail before generation.

## Mixed transports

A merged binary can hold both dialects: spec members (HTTP APIs from `base-url` + Swagger) and exec members (wrapped binaries from an `exec` block), which is what ships `ops forgejo` and `ops aws` as one binary. The driver sniffs each member's transport and both derive their binary name from `Group[0]`. Generated `main.go` dispatches per member through `specverb.Mount` or `execverb.Mount`, with spec-only imports gated behind a spec member's presence, so the binary compiles with either dialect alone or both.

An exec member carries no upstream spec and skips every spec-only seam: no spec lock, no fetch or skew, and no token, since the wrapped binary owns its credentials. Exec grants may add [embedded fixed files](specgen-materialization.md).

## The five verbs

- **`gen`** - render merged `main.go` into the cache, or `--out` to inspect it.
- **`lock`** - the deliberate online step. Per member it reads a vendored source or fetches upstream Swagger, **prunes to the granted surface**, and writes a deterministic gzip lock, then freezes the merged module graph in `specverb.lock`. `--umbra-ref` pins the framework version, `--umbra-replace` points at a local checkout.
- **`skew`** - prune live upstream to the granted surface and diff against each committed lock. Exit 3 on drift, and never write.
- **`build`** - materialize out-of-band and copy to `--out` (default `bin`) rather than exec it, following `go build -o`. `--set-version` stamps `--version` via `-ldflags`. Refuses without committed locks.
- **`run`** - materialize out-of-band and exec with passed-through args.

## Vendored sources

A spec member normally derives a live Swagger URL from `base-url`. A consumer may instead commit the contract beside its KDL member and name it with `spec`, which `lock` reads without reaching the endpoint. JSON, YAML, and `.gz` (decoded under a 128 MiB limit) are supported. Invalid or oversized gzip fails the lock: a present but unreadable vendored source is never permission to fetch the network copy, though a *missing* one may still fall back to the derived URL.
