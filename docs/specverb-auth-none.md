# `auth none`

```kdl
auth none
```

States that the upstream takes no credential. The block is required to stay
required - a spec that simply omits `auth` is a spec that forgot, and failing
closed on that is worth keeping.

## Why a fake credential is not a workaround

Before this, a credential-free upstream had to name a scheme anyway and supply
a placeholder:

```kdl
auth bearer {
    value literal "unused-public-storefront"
}
```

That reads as harmless, and for some upstreams it is. It is not harmless in
general, because the request then carries a **wrong** `Authorization` header
rather than none.

Measured against reddit's public Atom feeds, one request each from Go's
`net/http`:

| Headers | Response |
| --- | --- |
| a named `User-Agent` only | 429, the ordinary rate limiter |
| `User-Agent` + `Authorization: Bearer unused-public-feed` | 403 Forbidden |
| `Authorization: Bearer unused-public-feed` only | 403 Forbidden |

The placeholder is what earns the 403. An endpoint that serves anonymous
callers freely can still reject a caller presenting a credential it cannot
verify, and that rejection looks exactly like a block on the client.

## What it does

`authorize` returns without touching the request, the same as an unset scheme.
Nothing resolves, so no value provider runs and no secret is read.

`auth none` with a block is an error: a credential-free upstream resolves
nothing, so a `value` under it is a contradiction rather than a no-op.
