# provenance - where content came from

`pkg/provenance` records an origin claim about a piece of content, as a small
envelope a higher layer can act on. It answers *what do we know about this
content's origin, and how sure are we*. It never answers *is this content safe
to act on*.

## The four things a caller can conflate

Keeping these apart is the point of the package. They are separate defences and
none substitutes for another:

- **Command-construction safety** (`pkg/policy`) - argv cannot smuggle shell
  metacharacters into `execve`. Says nothing about who asked.
- **Execution isolation** - umbra performs **none**. It is an audit-and-gate
  framework, not a sandbox; see [architecture.md](architecture.md). Nothing here
  degrades under a jail, because there is no jail to degrade.
- **Provenance** (this package) - a claim about origin, and how far that claim
  was checked.
- **Application trust policy** - the consumer's own decision about which actors
  it will act on. Only the consumer holds it.

## The envelope

`Actor`, `Source`, `SourceID`, `ContentHash`, `ObservedAt`, `Verification`.
Every field is opaque: nothing here names an organization, forge, bot account,
or email domain, so a consumer can adopt it without inheriting a policy.

`ObservedAt` is when the content was *read*, not when it was authored. Only the
reader's clock is the reader's to trust.

## Ignorance is not trust

The zero value never reads as a pass:

- The zero `Verification` is `Unknown`, distinct from `Unverified`. Never
  running a check and running one that came up short are different facts.
- `Complete` names *every* missing field at once, so a caller fixes the
  envelope in one pass.
- `Trusted` requires a complete envelope **and** `Verified`. It is an input to
  the consumer's decision, not the decision.
- `CoversContent` re-hashes the bytes in hand, so the claim covers the content
  rather than some earlier revision of the same object.
