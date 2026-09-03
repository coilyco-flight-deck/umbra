# Upstream guardfiles

An **upstream guardfile** states policy about an MCP server a consumer fronts as a
proxy: where it is, what credential reaches it, and which of its tools may be
called. It mounts no CLI leaves and needs no tool lock, which is what separates
it from the [command shape](mcpverb.md).

```kdl
description "Search the published Tandem docs index."

mcp-upstream "ac.tandem/docs-mcp" {
    url "https://tandem.ac/mcp"
    transport streamable-http
    annotation-coverage partial annotated=7 silent=6

    auth header-token {
        header "Authorization"
        prefix "Bearer "
        value env "TANDEM_TOKEN"
    }

    can "search_docs"
    can "get_doc"
}
```

`mcpverb.ParseUpstream` reads it. `mcpverb.Classify` reports which shape a file
carries before either parser runs.

## Why its own top-level node

`wrap` reads every argument as a command path, so `wrap mcp upstream "x"` already
parses as the path `["mcp", "upstream", "x"]`. It does not fail, it means
something else, and telling the two apart would need `mcp` reserved as a first
argument, at which point a guardfile wrapping a CLI actually named `mcp` changes
meaning silently rather than refusing.

`mcp-upstream` is a sibling of `wrap` instead. A file carries one or the other,
never both, and `Classify` refuses a file carrying neither rather than picking a
default. `guardfile.Guardfile` is untouched, so no consumer reading `Group`
can be handed an upstream file and get an empty command path back.

## The body

* **`url`** - required, absolute `http://` or `https://`.
* **`transport`** - optional. `streamable-http` is the one value, and it is also
  the default, so stating it is a statement rather than a choice. Anything else
  refuses rather than reaching a consumer that speaks a different protocol.
* **`annotation-coverage <declared|partial|undeclared> annotated=<n> silent=<n>`** -
  optional. Whether the upstream declares `readOnlyHint` on every tool, some, or
  none. The counts are checked against the kind, so a hand edit cannot state
  `declared` beside a non-zero `silent`.
* **`auth`** - optional, in the [guardfile-wide auth grammar](specverb-policy.md):
  `header-token`, `bearer`, `query-param`, `none`, each resolving through a
  [value chain](value-providers.md) at call time rather than at parse.
* **`can "<tool>"`** - repeatable, one bare tool name. Not the `can call <tool>`
  of the command shape: there is no leaf to name and no guard to hang.

An empty allowlist parses. A guardfile that exposes nothing is still a statement
about where an upstream is and what credential reaches it.

## No tool schemas

The file carries policy and never contracts. A consumer snapshots the upstream's
tool schemas at connect time and fails closed on drift, so restating them here
would duplicate a source of truth that rots against a server nobody local owns.

## Siblings are the consumer's

umbra owns the `mcp-upstream` node and the `description` beside it. Every other
top-level node is the consumer's to accept or refuse, because which siblings a
proxy can honour depends on what that proxy projects. A server that silently
ignored an unprojected sibling would serve wider than the file reads, so a
consumer refuses what it cannot honour rather than dropping it.

## Not supported

`inherit` fails closed inside `mcp-upstream`. Textual flattening and the
one-shape-per-file rule have not been reconciled, and inheriting half an upstream
declaration would serve wider than either file reads.
