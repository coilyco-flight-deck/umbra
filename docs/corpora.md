# Corpora

A corpus is the [demo](demos.md) harness pointed at measurement instead of a camera.
`demos/corpora/<slug>/` holds a `corpus.kdl` call set, its guardfile under `.umbra/`,
and the generated `corpus.jsonl`, one row per call. The first one, `ask-tier`, is the
labelled set the Jev ask tier is measured against (umbra#8024).

```kdl
corpus ask-tier {
    tool git
    requires "a7f6c44b1fee1bb1642d28fa827025c19f8845c4"
    call "git log -- secret.txt" class=near-miss expect=reject
}
```

* `class` - the decision-boundary region the call was chosen for: `granted`, `never`,
  `withheld`, `uncovered`, or `near-miss`. It is the policy's intent.
* `expect` - what umbra did: `accept` or `reject` with an audit row, `unaudited` for a
  refusal that wrote none, or `help` for umbra's own help at exit 0. Generation fails
  when a call disagrees, so a behavior change forces a relabel instead of drifting.
* `note` - optional, for a row whose intent and outcome disagree on purpose.

`just corpus <slug>` regenerates the file. `just corpus-check <slug>` regenerates it and
fails unless the committed file matches byte for byte.

Every row names what produced it, because audit rows carry no version (umbra#7982).
`build` hashes the committed trees the shim compiles from, `generator` hashes
`demos/mint`, and `guardfile_sha256` hashes the policy. Generation refuses an
uncommitted build and a HEAD that lacks the `requires` commit. Only umbra's refusal
text is kept as `output`, since the wrapped tool's own output varies by its version.

Uncovered, `never run` and withheld refusals wrote no audit row until umbra#8121, so
the committed ask-tier corpus carries `audit: null` on those rows and the refusal text
is the evidence. A regenerated corpus attaches the reject row each one now writes.
