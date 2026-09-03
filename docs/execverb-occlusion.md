# occlusion primitives in the exec dialect

Three grant-level primitives in [the exec dialect](execverb.md) that state more than presence or absence. All three came down from mcp-beaver, where they were first needed, because none of them is MCP-shaped: they are about occlusion, and the rendering is the only part that differs between a tool listing and `--help` (umbra#6821).

## Stating an absence: `withhold`

Deny-by-absence is the default and stays the default. But absence carries four meanings at once - withheld by policy, not implemented, not offered by the binary, or simply not matched by the caller's search - and a caller reasoning from a hole has to guess which. The guesses go wrong both ways: a real capability gets worked around because it looked absent, or a workaround gets built for a restriction that was never there.

`withhold` converts one silence into a statement:

```kdl
withhold delete {
    reason "Deleting a repo exceeds what this audit trail can reconstruct."
    alternative "archive"
}
```

The verb mounts in the tree and appears in `--help`, with the usage leading `NOT AVAILABLE - withheld by policy.` so a reader scanning the first clause rules it out. Calling it fails `policy_denied`, carrying the reason and the alternative. **It reaches no binary and holds no credential**, so this is not a weakening of deny-by-absence: an undeclared verb is still absent, and a stub is a declared refusal rather than a narrowed grant.

`reason` is required. A stub that does not say why is the silence it was meant to replace, only louder.

Three things fail closed at build:

- A stub naming a verb the guardfile **grants**. It would advertise a working verb as refused, and the grant would look revoked with nothing revoking it.
- An `alternative` no grant mints, which sends a caller hunting for a verb they will never find.
- A stub beside `can run *` or an `allow` list. A funnel takes the whole group, so anything mounted under it is unreachable rather than merely redundant.

## Pinning a flag's value

`pin "<--flag>" "<value>"` fixes a flag the caller cannot change. umbra supplies it when the call omits it, and refuses the call when it passes a different value.

```kdl
can run ssm put-parameter {
    pin "--type" "SecureString"
}
```

A `when` guard gets refusal; a pin gets **correctness by construction**. For a flag whose only safe value is one constant, the second is better: nothing to type, nothing to get wrong, and the guardfile stops depending on every caller remembering (umbra#6821). Passing the pinned value is not a conflict, so a caller who spells it out agrees with the policy rather than tripping it.

A pin implies `value-flag`, since a pinned flag necessarily takes a separate token, so an `argN` guard beside one does not read the pinned value as a positional.

Pins apply **before** the gates and guards, so a `when` reads the argv that will actually run. That is the opposite of `resolve-flag` below, and deliberately: a pin changes *what* the call does, so a guard must see the change; a resolve-flag changes only how a value travels, so a guard must see what the caller typed.

`argv-prefix` is the neighbouring tool and a different one: it is wrap-level and prepends unoverridable leading argv, not a per-flag value.

## Resolving a flag's value instead of passing it through

`resolve-flag <name>` declares that umbra reads the flag's value rather than forwarding it. umbra resolves the caller's token through `pkg/valuesource`, writes the result to a mode-0600 file it owns, and hands the subprocess `file://<that path>`, unlinking it once the command exits. It implies `value-flag`, since a resolved flag necessarily takes a separate token.

```kdl
can run ssm put-parameter {
    resolve-flag "--value"
}
```

The caller's token selects the source: a `<provider>://` prefix naming a registered provider (`file://`, `env://`, or a consumer-declared one) resolves through it, and anything else is a `literal`. So `--value file:///path/to/token` is read and **trimmed**, and `--value s3cret` is spilled untrimmed but still leaves argv.

Two properties follow, and both were the point (umbra#6830):

- **The value goes through the trim policy the read path already uses.** `valuesource` trims a `file` or `env` source because a machine put the newline there, and leaves a `literal` intact because a reviewer can see it. Before this, `--value file:///path` was an opaque argv token the wrapped CLI opened itself, so umbra never saw the bytes and the provider that would have trimmed them was never called. A secret arriving 72 bytes stored at 72, and failed days later as a credential that reads correctly at a glance.
- **The value never sits in argv**, whichever form the caller used, so it stays out of `ps`, `/proc/*/cmdline`, and any argv-capturing audit row.

Resolution runs **after** every gate, guard, and flag-policy check, so a guard always reads what the caller actually typed rather than a path umbra just invented. It runs on the passthrough verb and on an action step, because a declared flag that bound on only one of the two paths would be a hole rather than a guard. A `--dry-run` plan resolves nothing and writes no file: it stays symbolic.

A source that cannot be read fails closed before any exec, and the error names the provider and address, never the value.
