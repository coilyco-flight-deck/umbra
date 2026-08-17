# User-Agent

Every request `opcore` fires carries `opcore.DefaultUserAgent` unless something
already set the header.

## Why a default rather than nothing

Go's own default is `Go-http-client/1.1`. Several public APIs refuse that
outright rather than rate-limit it, so a request built correctly in every other
respect is rejected for a reason nothing in the spec can express.

Measured against reddit, one request each, spaced to avoid confounding the
result with rate limiting:

| User-Agent | Response |
| --- | --- |
| `Go-http-client/1.1` | 403 blocked due to a network policy |
| none | 403 blocked due to a network policy |
| `Go-http-client/2.0` | 429, the ordinary rate limiter |
| a descriptive agent | 429, the ordinary rate limiter |

The block is on the anonymous client, not on the request. Anything that names
itself reaches the served path.

This is not reddit-specific. A descriptive agent is the stated etiquette for
most of the volunteer and nonprofit APIs a Guardfile reads, and an unnamed
client is bad manners against them whether or not they enforce it.

## What it does not do

It does not let a Guardfile choose the string. The inline grammar has no
`header` node, so an author who needs a contact address in the agent - which
some APIs ask for - still cannot state one. That half of umbra#303 stays open.

`Authorization` is out of scope for any future header node either way: `auth`
owns it, and a second path to it would be an unreviewed credential surface.
