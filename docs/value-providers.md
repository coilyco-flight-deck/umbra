# value providers, chains, and the `description` node

A `value <provider> <address>` source names *where* a value is read at request time. umbra ships three store-agnostic resolvers and nothing else: **`env`**, **`file`** (trimmed), and **`literal`**.

Anything store-backed is the consumer's, declared in the guardfile as a subprocess contract:

```kdl
provider ssm {
    exec aws ssm get-parameter --with-decryption --output text --query "Parameter.Value" --name
}
```

The address is appended as the final argument. Only stdout is read, trimmed; the resolved value never reaches argv, the audit row, or an error message, and a non-zero exit surfaces the exit status alone.

## Why exec rather than an SDK

umbra is a policy-free engine, and [architecture.md](architecture.md) keeps consumer-specific knowledge out of it. Linking a vendor SDK would put one cloud's credential-resolution rules inside the framework and hand every generated binary that dependency whether or not it resolves anything.

The trade is real and worth naming: credential resolution becomes whatever the declared binary does. A provider relying on SDK-specific behaviour, such as profile precedence or SSO fallbacks, inherits the CLI's behaviour instead, and that CLI must exist wherever the generated binary runs. The SDK path had its own version of this: aws-sdk-go-v2's default chain prefers static keys in `~/.aws/credentials` over an SSO profile of the same name, so a stale `[default]` silently shadowed SSO while the aws CLI resolved it, and only the SDK path broke.

## Fallback chains

KDL has no arrays, so an ordered fallback list sits in a children block, one source per line. Resolution takes the first source yielding a non-empty value with no error, so a fast local `env` can precede a durable store:

```kdl
value {
    env FORGEJO_API_TOKEN                    // fast local, checked first
    ssm "/forgejo/coilyco-ops/api-token"     // durable backup
}
```

Every field taking a `value` takes a chain: the three auth schemes and `base-url`. The inline form is a one-element chain, so existing Guardfiles are unchanged, and the two forms are mutually exclusive on one node. Parse-time refusals, never request-time: an empty block, a source missing its address, a mixed inline-and-block form, or a source carrying its own children.

`valuesource.ResolveFirst` skips a source when its provider errors **or** resolves empty, since success needs both. When every source fails it returns a combined error naming each provider and address tried but never a resolved value, so a provider handing back a value alongside an error still leaks nothing. A `--dry-run` stays offline, showing the chain symbolically as sources joined by ` | `, and describe names them the same way.

A `value` naming a provider that is neither built-in nor declared is an error at resolve time (`no provider registered for "ssm"`), never an empty string. Declaring a provider with no `exec`, or with an unknown child, is a parse error. Both dialects share the grammar, and a declaration in one member of a merged binary serves the others.

## The top-level `description` node

Every `.kdl` spec may carry a first-class top-level `description "..."` node, sibling of the root block and present on both dialects. It is **queryable contract data rather than a comment header**, the sanctioned home for the standing context a comment header used to carry.

A single string argument, with KDL's escaped and multi-line literals available for multi-paragraph prose. An empty `description ""` fails closed, so the node is never a silent no-op. It holds the durable what and why a reader needs to understand the surface; changelog and provenance archaeology belong in maintained documentation instead.
