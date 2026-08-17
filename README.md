# umbra

a config driven occlusion framework for your CLIs and APIs

## About

Designed to sit between AI agents (or any semi-trusted automation) and the host system, featuring:

- argv validation rejecting shell metacharacters before they reach `execve`
- append-only JSONL audit log with lumberjack rotation
- read / write / delete scope tokens, validated per verb
- best-effort RepoRoot stamping that records each audit row's git toplevel (empty outside any repo)
- clean+synced gate refusing repo-shaped verbs on a dirty tree
- per-repo config loaded from a consumer-chosen YAML filename (e.g. `.<app>/<app>.yaml`)
- thin pass-through wrapper for embedding existing CLIs as audited subcommands
- per-invocation CONNECT proxy with consumer-supplied egress allowlist
- public exit-code taxonomy for orchestrators

The repository also ships **`specgen`**, an installable no-code driver that turns
KDL policy plus committed locks into standalone guarded CLIs without hand-written
Go. It discovers `.specgen/` and can generate, lock, check skew, build, and run.
An explicit `--skills-out` path also renders a concise native agent skill plus
a lazy command index from the merged command tree.

## Install specgen

Homebrew users on macOS or Linux can install from the coilyco tap:

```sh
brew tap coilyco-flight-deck/tap https://forgejo.coilysiren.me/coilyco-flight-deck/homebrew-tap.git
brew install coilyco-flight-deck/tap/specgen
```

Scoop users on Windows can install from the coilyco bucket:

```sh
scoop bucket add coilyco https://forgejo.coilysiren.me/coilyco-flight-deck/scoop-bucket.git
scoop install coilyco/specgen
```

Tagged Forgejo releases also publish raw `specgen` binaries for Linux, macOS,
and Windows on amd64 and arm64, plus `SHA256SUMS`. Go users can install directly:

```sh
GOPRIVATE=forgejo.coilysiren.me go install forgejo.coilysiren.me/coilyco-flight-deck/umbra/cmd/specgen@vX.Y.Z
```

`specgen --version` reports both the installed driver version and the
umbra module ref that `lock` will freeze by default. The driver invokes the
Go toolchain when it resolves locks and builds generated CLIs.

## Documentation

See the [`specgen` guide](docs/specgen.md), [`docs/FEATURES.md`](docs/FEATURES.md) for a feature inventory, and [`examples/`](examples/) for runnable demos one per primitive. `make docs-serve` renders the documentation and CLI reference locally. Other development verbs also run through the [`Makefile`](Makefile).

## Support

If you found a bug or have a feature request, [create a new issue]. Participation in this community is governed by the [Code of Conduct](CODE_OF_CONDUCT.md). Security disclosures go through [SECURITY.md](SECURITY.md).

Sibling repo: [mcp-beaver].

### License

See [`LICENSE`](./LICENSE).

[create a new issue]: https://forgejo.coilysiren.me/coilyco-flight-deck/umbra/issues/new
[mcp-beaver]: https://forgejo.coilysiren.me/coilyco-flight-deck/mcp-beaver

## See also

- [AGENTS.md](AGENTS.md) - agent-facing operating rules.
- [docs/FEATURES.md](docs/FEATURES.md) - inventory of what ships today.

Cross-reference convention from the shared repo-pointer rule in the agentic-os docs.
