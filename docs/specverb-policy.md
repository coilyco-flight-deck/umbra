# specverb policy: auth, deny, restrict, tiering

The policy surface a Guardfile authors on top of the `op`-bound grants. Engine and layering in [specverb.md](specverb.md).

## Value sources and auth

A secret or opaque host is named, never committed: `value <provider> "<address>"`. umbra never reads the store; a registered provider does, so store clients stay in the consumer. An unregistered provider fails closed at request time rather than yielding a silent empty secret. See [value providers](value-providers.md).

Three auth schemes, each redacting its secrets in `--dry-run`: `header-token { header; prefix; value ... }` (the trailing space in `prefix "token "` is significant), `bearer { value ... }`, and `query-param { param key { value ... } ... }` for a dual-secret form injected as query parameters. Describe names the scheme and its value sources, never the value.

`auth none` states that the upstream takes no credential: `authorize` returns without touching the request, so no provider runs and no secret is read. The block stays **required**, because a spec that simply omits `auth` is a spec that forgot, and failing closed on that is worth keeping. `auth none` carrying a block is an error, since a `value` under it is a contradiction rather than a no-op.

A placeholder credential is not a substitute. A credential-free upstream that named a scheme anyway and supplied `value literal "unused"` sent a **wrong** `Authorization` header rather than none, and measured against reddit's public feeds that placeholder is exactly what earned a 403 where a named `User-Agent` alone reached the ordinary rate limiter. An endpoint serving anonymous callers freely can still reject one presenting a credential it cannot verify, and that rejection looks just like a block on the client.

`base-url` takes a committed string or a block resolving the host through a provider at request time, for a tailnet-only host that must not be committed. It resolves lazily on the first real request and caches, so mounting the tree never touches the store, and `--dry-run` stays offline. The forms are mutually exclusive and a spec member must carry one. With no committed host no spec fetch URL is derivable, so the spec is vendored beside the guardfile.

## Deny beats allow

`cannot`/`never <verb> <resource>` blocks that class and beats any matching `can`. The allowed leaf is dropped from the mounted tree, the spec lock, and the action poll set, replaced by a teaching leaf failing closed with a `PolicyDenied` exit carrying the grant's `message`. A deny with no matching allow still mounts that leaf, so an operator reaching for a blocked verb learns why rather than hitting "unknown command".

## Restrict: the scope gate

`restrict <param> matches "<glob>"...` is a wrap-level allowlist. Every leaf whose path template carries `{param}` must supply a matching argument at invocation or fail closed with `PolicyDenied` before any wire call. A malformed glob matches nothing. Enforced on the direct leaf path and the action poll/call path alike.

## inherit and override

A wrap may carry `inherit "<path>"` directives pulling in another guardfile's grants, so a tiered surface composes by layering rather than copy-paste. Resolution is **textual** and runs before the typed parse: each file is flattened recursively, its wrap body spliced in, and the directives dropped, after which the ordinary pipeline runs unchanged. Effective grants are the union, order-independent since precedence resolves by class rather than position. `restrict` inherits deduped by param with the child winning, singletons (`spec`, `base-url`, `auth`) inherit only when the child declares none, a restated grant collapses keeping the child's body, and `action` blocks stay child-local. A missing ref or a cycle fails closed with a teaching error.

The load-bearing rule: **an inherited `never` beats a plain `can`, and the only construct that beats an inherited `never` is an `override` in a higher tier naming the exact verb+resource.** The posture is deny low, override high. A higher tier may write `can delete "*"` and an inherited `never delete issue` still carves `issue` out silently, by design; deny is sticky upward.

`override can <verb> <resource>` is the sole escalation and re-grants exactly that pair, rejecting `"*"`, so every escalation a tier holds is enumerated and reviewable. Enforced when guardfiles flatten rather than silently at runtime: a plain explicit `can` shadowed by an inherited `never` is a build error pointing at `override`, and an `override` lifting no matching `never` is one too, since silently it would be a plain `can` - the fail-open the keyword exists to stop.
