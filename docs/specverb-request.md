# specverb request semantics

How the generic action behind every mounted leaf assembles, previews, and fires its request. Engine and policy layers are in [specverb.md](specverb.md).

## Inputs

- **Path params** become positional args, count-validated before any wire call. **Query params** become typed flags: scalars encode once, arrays as repeated keys in input order, unset values omitted.
- **Body fields** become typed flags; an unset optional is omitted rather than sent as a zero value, and arrays repeat the flag. **`--body-file`** supplies the whole body instead. Required fields are enforced at assembly rather than in the CLI layer, so either source satisfies them.

A local input shadowing a reserved engine flag (`--dry-run`, `--query`, `--output`, `--body-file`), or a query/body collision on one leaf, refuses to build rather than shadowing silently.

## The shell-metachar gate is location-aware

`verb.Wrap` → `policy.ValidateArg` refuses shell metacharacters, but only on inputs composing into the request **URL**, the injection surface. Path params and query flags stay gated, each element of a repeated parameter independently. Body and form fields and `--body-file` are encoded into the body and never reach a shell or the URL, so they are exempt: gating them mangled legitimate free text.

## Firing

Auth resolves the secret through the value-provider registry. `--dry-run` prints the resolved request with the secret redacted and fires nothing. Live responses render through the `--query`/`--output` rail, and an empty 2xx prints an `ok:` line. The client refuses redirects for mutating methods, so a renamed target cannot silently swallow a write.

Every request carries `opcore.DefaultUserAgent` unless the header is already set. Naming your client is the stated etiquette for the volunteer and nonprofit APIs a Guardfile reads. It is etiquette rather than a fix: an earlier measurement blamed reddit's 403 on Go's default agent, but re-measured from `net/http` every agent reaches the served path, and the 403 came from the placeholder `Authorization` a credential-free Guardfile had to send. A Guardfile cannot yet choose the string, the open half of umbra#303.

## Non-JSON responses

An operation whose success response offers no JSON writes its body to stdout byte for byte. The spec-driven path infers this from the declared media type; a response listing JSON beside something else is negotiating content rather than declaring bytes, so it is parsed. An inline grant says it outright with a bare `raw-response` node, fail-closed against an argument, block, duplicate, or a `fail-when` pairing with nothing to evaluate.

Both paths choose **before** firing rather than after reading, the whole of umbra#289: decoding first fails on a plaintext or ZIP body, leaving the raw branch unreachable. Only the decode is skipped, never a gate, and `--query` is refused rather than ignored.

Fixed non-Swagger leaves live in [fetch overlays](specverb-fetch.md).
