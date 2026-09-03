# umbra

umbra is a security-boundary framework for [urfave/cli](https://github.com/urfave/cli) v3
applications, sitting between AI agents (or any semi-trusted automation) and the host system.
What you did not declare does not get through. Policy lives in a KDL guardfile rather than in
code, enforced across two surfaces: `cli/` around subprocess exec, `http/` around outbound requests
(HTTP APIs and MCP servers alike).

## Start here
- [Getting started](getting-started.md) - install it, then watch a refusal.
- [Features](FEATURES.md) - the inventory of what ships today.
- [Examples](https://forgejo.coilysiren.me/coilyco-flight-deck/umbra/src/branch/main/examples) - one runnable app per primitive.

## Concepts
- [Architecture](architecture.md) - the two guarded surfaces and the shared core.
- [Spec-driven verbs](specverb.md) - the three-layer engine behind the HTTP surface.
- [Exec-dialect verbs](execverb.md) - the same grammar aimed at wrapped binaries.
- [MCP-dialect verbs](mcpverb.md) - the same grammar aimed at upstream MCP servers.

## Guides
- [The no-code driver](umbra-cli.md) - author policy and locks, never Go.
- [Materialization](umbra-materialization.md) - how `run` and `build` cache a generated binary.
- [Fetch overlays](specverb-fetch.md) - mount fixed HTTP leaves straight from the guardfile.

## Reference
- [Policy](specverb-policy.md) - auth, deny, restrict, tiering.
- [Op resolution](specverb-resolution.md) - verbs, wildcards, unrecognised shapes.
- [Request semantics](specverb-request.md) - how a mounted leaf assembles and fires.
- [Complex actions](specverb-actions.md) - composite verbs and their five invariants.
- [Describe model](specverb-describe.md) - generated visibility for a generated surface.
- [Inline operations](opcore-inline.md) - descriptors stated directly in KDL.
- [Body projection](opcore-body.md) - `map`, `set`, and pinned values.
- [Value providers](value-providers.md) - `env`, `file`, `literal`, and minted tokens.
- [Upstream guardfiles](mcpverb-upstream.md) - `mcp-upstream`, the proxied-server shape.
- [MCP Apps host](mcpapps.md) - the frames a rendered widget sends back, under the guardfile.

## Contributing
- [Contributing](CONTRIBUTING.md) - how to propose a change.
- [Release pipeline](release-pipeline.md) - Forgejo-canonical publication and the mark.

Sibling repo: [mcp-beaver](https://forgejo.coilysiren.me/coilyco-flight-deck/mcp-beaver).
