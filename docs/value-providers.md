# value providers

A `value <provider> <address>` source names *where* a value is read. umbra ships
three store-agnostic resolvers and nothing else:

- **`env`** - read an environment variable
- **`file`** - read a file, trimmed
- **`literal`** - the value inline

Anything store-backed (a parameter store, a secret manager, a tailnet) is the
consumer's, declared in the guardfile as a subprocess contract:

```kdl
provider ssm {
    exec aws ssm get-parameter --with-decryption --output text --query "Parameter.Value" --name
}
```

The address is appended as the final argument, so the block above resolves
`value ssm "/forgejo/coilyco-ops/api-token"` by running:

```
aws ssm get-parameter --with-decryption --output text --query Parameter.Value --name /forgejo/coilyco-ops/api-token
```

Only stdout is read, trimmed. The resolved value never reaches argv, the audit
row, or an error message; a non-zero exit surfaces the exit status alone.

## Why exec rather than an SDK

umbra is a policy-free engine, and [architecture.md](architecture.md) keeps
consumer-specific knowledge out of it. Linking a vendor SDK would put one
cloud's credential-resolution rules inside the framework and hand every
generated binary that dependency whether or not it resolves anything. An exec
contract keeps the vendor entirely in the consumer's guardfile.

The trade is real and worth naming: credential resolution becomes whatever the
declared binary does. A provider that relied on SDK-specific behaviour (profile
precedence, SSO fallbacks) inherits the CLI's behaviour instead, and that CLI
must exist wherever the generated binary runs.

## Fail-closed

A `value` naming a provider that is neither a built-in nor declared is an error
at resolve time (`no provider registered for "ssm"`), never an empty string.
Declaring a provider with no `exec`, or with an unknown child node, is a parse
error.

Both dialects share the grammar: an exec-dialect `wrap` and a spec-dialect
`wrap` each accept `provider` blocks, and a declaration in one member of a
merged binary serves the others.

## See also

- [architecture.md](architecture.md) - why config and vendor knowledge stay out of the core.
- [specverb-policy.md](specverb-policy.md) - where `value` sources appear in a spec guardfile.
