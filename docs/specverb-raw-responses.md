# Declared non-JSON responses

An operation whose success response declares a non-JSON media type, `text/plain`
for a CI job log or `application/zip` for a run archive, writes its body to
stdout byte for byte instead of parsing it. `Descriptor.RawResponse` carries the
decision, and the engine sets it from the resolved spec's declared media type.

Nothing in the Guardfile declares this, and nothing needs to. A spec that
declares no media type stays parsed, which is the fail-safe direction: a body
that turns out not to be JSON fails loudly rather than passing through unchecked.

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
postcondition has nothing to evaluate and does not run.

## Not covered here

The inline grammar has no node that sets `RawResponse`, so a `.mcp.kdl` author
cannot request it on a hand-written grant. Only the spec-driven path infers it.
That is part two of umbra#289 and is still open.

## See also

* [specverb-request.md](specverb-request.md) - the generic request action.
* [specverb-fetch.md](specverb-fetch.md) - the raw-stdout overlay sibling.
