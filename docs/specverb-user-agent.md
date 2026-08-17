# User-Agent

Every request `opcore` fires carries `opcore.DefaultUserAgent` unless something
already set the header.

## Why a default

Naming your client is the stated etiquette for most of the volunteer and
nonprofit APIs a Guardfile reads. An unnamed one is bad manners against them
whether or not they enforce it, and some do enforce it.

## Correction: this is not what reddit was blocking

The change originally landed on a measurement that did not support it. The
first round used `curl`, where `Go-http-client/1.1` and an empty agent both
answered 403 while any other agent reached the rate limiter - and that was
generalised to Go's client, which is the one that matters.

Re-measured from Go's `net/http`, every agent including Go's own default
reaches the served path. reddit's 403 came from the placeholder
`Authorization` header a credential-free Guardfile was forced to send, not from
the agent. That is fixed by [`auth none`](specverb-auth-none.md), and the
measurements are recorded there.

The default agent stays, on the etiquette argument alone. It is recorded here
as etiquette rather than as a fix so nobody later reads it as one.

## What it does not do

It does not let a Guardfile choose the string. The inline grammar has no
`header` node, so an author who needs a contact address in the agent - which
some APIs ask for - still cannot state one. That half of umbra#303 stays open.

`Authorization` is out of scope for any future header node either way: `auth`
owns it, and a second path to it would be an unreviewed credential surface.
