# Value flags

`valueFlags` in `cli/execverb/argv.go` names the long flags whose value arrives
as a separate argv token. Without it `--region us-east-1` leaves `us-east-1`
looking like a positional, where it can slip past an `argN` guard that was
written to bound the real first argument.

## It is one vendor's shape

The table is still drawn from a single vendor's CLI. It belongs in the
guardfile, declared by the spec that knows its own binary, rather than in the
engine that runs every binary.

Until it moves, dropping an entry silently weakens any `argN` guard on a binary
that takes that flag. Silently is the problem: nothing fails, the guard simply
starts bounding the wrong token. Tracked as umbra#282.

## See also

* [execverb](execverb.md) - the grammar and the engine around it.
