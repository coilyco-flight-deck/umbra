# specgen materialization: cache, embedded files, skills

`run` and `build` materialize the generated consumer binary out-of-band. The consumer keeps policy and locks in source control and never commits generated Go or build glue.

The materialized module lives under `config.CacheDir()`: generated `main.go`, the `go.mod` and `go.sum` replayed from `specverb.lock`, each member's embedded inputs, and the compiled binary. The cache key is the generated binary name plus the sorted root-relative member identities, so one project caches both its source build and a renamed `--binary` build, and an identical tree moved to another absolute location keeps its cache identity.

`.stamp.json` records input hashes for the member identities and bytes, decoded spec contracts, dependency lock, generator version, and version stamp. A rebuild fires only when one of those changes or the binary is missing. `run` refuses without committed locks rather than silently locking, and `lock` is the only online dependency-resolution step. First `run` after a fresh `lock` works offline, because `lock` already warmed the module cache.

## The cache lock, and where it does not exist

Materialize+build runs under an advisory lock on `<cache>/.lock` via `pkg/flock`, so two concurrent runs against one cache dir serialise rather than race. That lock is **unix-only**. Elsewhere specgen prints to stderr that it is building unserialised and continues, rather than reporting a lock it never took:

```
specgen: no cache lock on windows, building <dir> unserialised (a concurrent run may race)
```

Continuing is deliberate: specgen ships Windows binaries, the build is idempotent, and a concurrent run against one cache dir is rare. Being quiet about it was not.

## Embedded fixed files

An exec grant may `embed "scripts/x.py"` a reviewed file into the binary and place its absolute runtime path at a fixed argv position, so complex logic needs no repository checkout or relative runtime path. `argv` fragments and `embed` nodes append in declaration order, and help and describe show `<embedded:scripts/x.py>` rather than the temporary path. `embed` counts as a pinned argv override, so a grant holding only an embedded file can be `sealed`.

The source path is relative to the declaring guardfile and must be normalized, portable, and confined to that directory. Absolute paths, `..`, backslashes, symlink escapes, missing or non-regular files, and artifact collisions fail the build, and one file is limited to 4 MiB. Specgen reads the source during discovery, includes its identity and bytes in the cache hash, and emits a `go:embed`, so changing only content still rebuilds. At runtime the binary writes embedded files beneath one private temporary directory with owner-only permissions, execverb fails closed on a missing or non-absolute reference, and the caller cannot select, replace, or reorder that path. The directory lives only as long as the process.

## Generated skills

`--skills-out <root>` is opt-in; ordinary verbs write no skill or Markdown into the consumer tree. The selected binary writes `<root>/<binary>/SKILL.md` with frontmatter and a short orientation, plus `references/commands.yaml` listing every reachable leaf, summary, and canonical flag from the merged urfave tree. Identical specs, locks, binary names, and generator versions produce identical paths and content.

The eager `SKILL.md` stays deliberately small: it tells an agent to start with `--help`, follow group help, and use `describe`. The lazy index makes every leaf discoverable without copying exhaustive reference prose into startup context. The running CLI remains authoritative, and the skill grants no permission, resolves no credential, and does not replace runtime policy. Mixed spec and exec members contribute to one index under the shared runtime binary.
