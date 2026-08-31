# the MCP Apps host (mcpapps)

An MCP App is untrusted remote HTML holding a JS bridge that can call tools. `pkg/mcpapps` is the host half of that bridge: it answers the frames a view sends over `postMessage`, and forwards a view-initiated `tools/call` only when the guardfile declared it.

```kdl
wrap aosguard ops monitor {
    mcp stdio { command "monitor-server" }
    can call get_system_info {
        widget {
            can call poll_system_stats {
                deny scope matches "^secret"
            }
            never call reboot_host
            can read "^ui://"
        }
    }
    can call wipe_disk { destructive }
}
```

`get_system_info` returns a view. That view may call `poll_system_stats` and read a `ui://` resource, and nothing else. `wipe_disk` is a CLI leaf in the same guardfile and the view still cannot reach it: a `widget` block is its own surface rather than an inherited one.

Runnable: [`examples/mcpapps/`](../examples/mcpapps/main.go) replays a real widget's frame sequence against a server it starts itself.

## Why a widget needs its own policy

A measured widget fired **eleven unprompted `tools/call` frames** in the first few seconds of rendering, and the prototype host answered all of them without asking anything ([inbox#505 comment 82130](https://forgejo.coilysiren.me/coilysiren/inbox/issues/505)). The spec's answer is a sandboxed iframe plus per-call consent. Consent at that rate is not consent.

So the grant is declarative and the default is nothing:

- **A grant with no `widget` block gates nothing in.** `WidgetPolicy` returns a nil `Policy`, and a `Host` with a nil `Policy` refuses every view-initiated call. An undeclared widget renders and reaches no upstream.
- **`can call *` is a parse error inside `widget`**, though the outer block takes one. A wildcard over a view is the unconditional forwarding this block exists to replace.
- **`tools/list` answers from the grant, not the upstream.** A view told about a tool it may not call is invited to make a call that will refuse. Same deny-by-absence rule as [descriptors](specverb-descriptors.md).
- **Reads are deny by absence too.** With no `can read` sentence a view reads nothing, and a `never read` match refuses. A `can read` pattern is an unanchored regex over the whole URI, like every other guard here.

Guards inside a `widget` grant are checked against the **called** tool's locked schema, not the instantiating one's, and a selector that tool does not take fails the build. A guard matching nothing reads like a guard that passed.

## A refusal is a reply

Every request frame gets exactly one reply. A refused call comes back as a JSON-RPC error carrying the frame's own id, code `-32001`, and a message naming the tool and the sentence that would permit it:

```
policy_denied: tool "wipe_disk" is not granted to this view;
add `can call wipe_disk` to its `widget` block
```

Dropping the frame instead would leave the view waiting on a reply that never arrives, which is the same silent-failure class as the traps below. `mcpverb` guards reuse `opcore.CheckProxyRules`, so a view call and a CLI leaf refuse on identical regex semantics rather than two implementations that drift.

## Four ways this fails silently

Each of these produces no error frame and no console warning: a widget that renders and does nothing. All four are encoded in the package rather than left to a caller.

1. **`ui/notifications/initialized` is View to Host**, not Host to View. The view emits it when its SDK is ready, and it is the correct trigger for pushing the instantiating tool result.
2. **`McpUiInitializeResult` requires both `hostCapabilities` and `hostContext`.** Omit either and the view's SDK never becomes ready, dropping every later notification without complaint.
3. **`hostCapabilities.serverTools` is load-bearing.** Omit it and the view will not send `tools/call` at all.
4. **`postMessage` is structured clone, not JSON**, so a key holding an empty value survives the hop. A reply carrying `error` beside a valid `result` reads as a failure, and the widget renders `Error` while receiving correct data. `Reply` has unexported members and three constructors, so both cannot be set.

Field-level detail and the frame log they came from: [inbox#505 comments 82102 and 82130](https://forgejo.coilysiren.me/coilysiren/inbox/issues/505). The contract itself was read from `spec.types.d.ts` in `@modelcontextprotocol/ext-apps`, not inferred from behaviour.

## What umbra ships, and what it does not

`Host` is transport-free: a caller reads a frame from wherever it arrives and hands the bytes to `Handle`, which returns the replies to post back. umbra ships no presenter, no iframe, and no HTML page.

That leaves the **isolation-versus-automation** decision with the consumer, and it is a real one. Serving the host page over `http://` lets the iframe run under `sandbox="allow-scripts"` alone, which is what the spec's model wants. The cost is that an agent's accessibility tree stops at `Iframe [ref=e1]`, so verifying a widget falls back to pixels; under `allow-same-origin` it read `button "Start" [ref=e2]`. **Take the isolation.** `allow-same-origin` on a page holding the bridge gives untrusted remote HTML the host's origin, and a verification convenience is not worth it. [umbra#349](https://forgejo.coilysiren.me/coilyco-flight-deck/umbra/issues/349) holds what is still open.

## Not yet

- `_meta.progressToken` rides on view-initiated calls and is passed through rather than honoured, so a long call reports no progress.
- `ui/open-link` and `ui/download-file` are unimplemented, and deliberately undeclared in `hostCapabilities`: a view asking for one gets `-32601` rather than a capability the host would not honour.
- `resources/list` answers empty. This host grants resources one read at a time rather than as a browsable list.

## See also

- [mcpverb.md](mcpverb.md) - the dialect this extends, and the lock that preserves `_meta`.
- [specverb-descriptors.md](specverb-descriptors.md) - the deny-by-absence rule the widget surface inherits.
- [mcpverb-serving.md](mcpverb-serving.md) - projecting the same grants into a served surface.
