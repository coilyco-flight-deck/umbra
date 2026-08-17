# unrecognised verbs, and why the guess is now audible

`MethodForVerb` maps a grant's verb to an HTTP method by convention. A verb it
has never seen falls through to **POST**, reading the trailing noun as a child
sub-collection to create on the resource.

## Why this one rule needed a signal

Everywhere else the grammar refuses to infer authority. An unwritten grant is an
unminted tool, a `can` body rejects an unknown node fail-closed, and a fetch
overlay must state its `output`. A verb it has never seen receiving a mutating
method by default was the odd one out.

The failure is silent in the dangerous direction. A wrong GET fails loudly at
the API. A wrong POST against a real endpoint may not.

## What is audible now

The fallthrough still resolves, so no guardfile breaks. What changed is that
nothing throws the signal away:

- `MethodForVerb` returns `ok=false` for the fallthrough, as it always did.
- The inline parser records it on `Descriptor.MethodInferred` instead of
  discarding it.
- `opcore.ParseInlineWithWarnings` returns one note per inferred grant, naming
  the verb and the method it was given.

## Stating the method instead

For a novel verb, declare it rather than let it be guessed:

```kdl
can transfer repo {
    path "/repos/{owner}/{repo}/transfer"
    method "PUT"
}
```

A stated method suppresses the inference and owes no warning. A stated `DELETE`
marks the leaf destructive whatever the verb is called, because the
confirmation gate keys off the effect rather than the spelling.

`comment` and `pin` were the only verbs in the fleet's guardfiles reaching POST
through the fallthrough. Both are now in the convention table, so the mapping is
a decision rather than an accident.
