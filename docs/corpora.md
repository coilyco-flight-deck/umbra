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

Every accept and reject row declares `rule=`, the one guardfile line meant to decide
it: `can run`, `never run`, `withhold`, `deny-flag`, `deny-when`, or `uncovered` for
the default refusal. The run fails when the audit verb or refusal text names another
line, so a consumer dropping one rule at a time can trust the pairing. Since umbra#8121
every refusal writes its reject row, so only umbra's own help carries `audit: null`.

The ask-tier set spans eight tools, one corpus and one guardfile each
(`ask-tier-<tool>`): git, go, cargo, helm, terraform, uv, npm and openssl. Each
grants only leaf paths, so no `can run` parents another, and refuses real verbs with
`never run`, never a nested path. `env` sets a corpus offline, and `tool-version`
pins the real tool's version line: generation refuses a host that differs, and the
line lands on every row as `tool_version`.
