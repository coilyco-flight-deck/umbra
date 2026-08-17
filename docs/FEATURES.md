# umbra features

Inventory of umbra today, grouped by **guarded surface** over a shared `pkg/`. See [architecture.md](architecture.md); each primitive ships a runnable `examples/<name>/`.

## CLI passthrough surface (`cli/`)

- **passthrough** - Thin wrapper embedding an existing binary (aws, gh, kubectl) as an audited urfave subcommand. See [passthrough.md](passthrough.md).
- **execverb** - Exec-dialect KDL verbs plus the `passthrough <bin>` funnel, complex actions, and inspect lists. See [execverb.md](execverb.md).
- **verb** - Middleware wrapping every `*cli.Command.Action` in the standard validate -> execute -> audit pipeline.
- **shell** - Subprocess exec with audited argv, stderr tail, and env injection.
- **gittree** - Clean+synced gate refusing repo-shaped verbs on a dirty tree.
- **repocfg** - Per-repo config loaded from a consumer-chosen YAML filename.

## HTTP request surface (`http/`)

- **egress** - Per-invocation CONNECT proxy with a consumer-supplied allowlist, in enforce or observe mode.
- **specverb / guardfile** - Spec-driven verbs: [resolution](specverb-resolution.md), [policy and tiering](specverb-policy.md), [requests](specverb-request.md), [actions](specverb-actions.md), [describe](specverb-describe.md), [fetch overlays](specverb-fetch.md).
- **specgen / codegen** - The no-code driver: discovery, locks, generation. See [specgen.md](specgen.md) and [materialization](specgen-materialization.md).
- **opcore** - The frozen inline grammar: typed query, nested-string body projection, JMESPath postconditions, MCP proxy grants. See [opcore-inline.md](opcore-inline.md).
- **Named client** - every request carries a default User-Agent as etiquette, and `auth none` states a credential-free upstream rather than faking one. See [policy](specverb-policy.md).
- **respfmt** - JSON renderer with optional JMESPath projection and five output formats (yaml, yaml-stream, json, text, table), mirroring the aws CLI `--query` / `--output` surface.

## Shared core (`pkg/`)

- **audit** - Append-only JSONL invocation log with lumberjack rotation and optional typed CI attribution. The package preserves that context but does not establish its trust.
- **policy** - Argv validation rejecting shell metacharacters before they reach `execve`.
- **scope** - Resolve cwd to its git toplevel, best-effort, stamping each audit row's RepoRoot (empty outside any repo).
- **exitcode** - Public exit-code taxonomy (success / generic / policy-denied / upstream-failed / internal / user-error) for orchestrators.
- **valuesource** - Shared `value <provider>` resolution with ordered fallback chains. See [value providers](value-providers.md).
- **config** - Layered-config primitives plus a generic `OverlayFile[T]`.
- **stepflow** - Transport-agnostic ordered sequence engine with explicit data threading.
- **ttlcache** - Generic TTL-keyed cache. **skillgen** - Render deterministic agent skills from CLI command trees.
- **broker** / **credseed** / **provenance** - Credential broker, env seeder, and origin envelope. See [broker.md](broker.md).
- **scan** / **attribution** / **flock** / **version** / **issueref** / **ownertrust** - Ward-lifted helpers. See [ward-helpers.md](ward-helpers.md).

## Repo development

`Makefile` is the source of truth for dev verbs, since umbra is deliberately unguarded. `.golangci.yaml` and `staticcheck.conf` mirror urfave/cli, and CI validates code, secrets, and docs. Release is automated and Forgejo-canonical; see [release-pipeline.md](release-pipeline.md).

## See also

- [README.md](../README.md) - human-facing intro.
- [AGENTS.md](../AGENTS.md) - agent-facing operating rules.

Cross-reference convention from the shared repo-pointer rule in the agentic-os docs.
