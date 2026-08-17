# Declared non-JSON responses

An operation whose success response offers **no JSON at all** - `text/plain`
for a CI job log, `application/zip` for a run archive - writes its body to
stdout byte for byte instead of parsing it.

A response listing JSON beside something else is negotiating content rather
than declaring bytes, so it is parsed. That matters because a Swagger 2.0
shared `$ref` response inherits the document's root `produces`, and Forgejo's
root lists `text/html`: reading "any non-JSON type" as raw refused `--query` on
every object read in the fleet. See umbra#293. `Descriptor.RawResponse` carries the
decision, and the engine sets it from the resolved spec's declared media type.

A spec that declares no media type stays parsed, which is the fail-safe
direction: a body that turns out not to be JSON fails loudly rather than
passing through unchecked.

## Declaring it by hand

The inline grammar has no media type to infer from, so a `.mcp.kdl` grant says
it outright with a bare `raw-response` node - the reddit-over-Atom shape, where
there is no spec at all:

```kdl
can list post {
    path "/r/{sub}/new.rss"
    raw-response
}
```

Bare and fail-closed, matching its siblings. An argument, a property, a block, a
duplicate, or pairing it with `fail-when` is a parse error rather than a silent
passthrough. That last one because a raw body is never decoded, so the
postcondition would sit inert instead of guarding anything.

## The choice precedes the request

Both execution paths decide before firing rather than after reading. That
ordering is the whole fix for umbra#289.

Decoding first and reacting to the failure cannot work, because the decode error
is indistinguishable from a real one. A log opening on a timestamp fails with
`invalid character '-' after top-level value` and a ZIP fails on its `PK` magic,
and both used to abort the call before either raw branch was consulted. The
branches existed and were unreachable, so setting the flag changed nothing.

`Runtime.FireCaptureRaw` is the transport half. It shares the request, auth, and
status handling of `FireCapture` and returns the body undecoded.

## What is unchanged

Only the decode is skipped, never a gate. The restrict scope gate, auth, the
redirect floor, and the non-2xx failure path all run exactly as they do for a
JSON leaf, and an upstream error is still a coded `upstream_failed` rather than
error-page bytes handed back as a log.

`--query` is refused rather than ignored, because a JMESPath projection over
bytes that are not JSON has no meaning. An empty body stays a success, since a
job that produced no output is a real state.

A raw operation has no decoded value, so an inline grant's `fail-when`
postcondition has nothing to evaluate. Declaring both is refused at parse time
rather than leaving one of them quietly inert.

## See also

* [specverb-request.md](specverb-request.md) - the generic request action.
* [specverb-fetch.md](specverb-fetch.md) - the raw-stdout overlay sibling.
