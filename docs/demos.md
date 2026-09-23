# Demos

Each directory under [`demos/`](../demos/README.md) is one umbra demo. It is a guardfile over a
real tool, the commands typed on camera, and the exit code and output each command
must show. A demo is a test first and a clip second, so a clip never shows a refusal
that verify did not check.

A demo holds structure only. Its words come from umbra's own refusals and from the
guardfile's `reason` strings, never from prose written for the clip.

## Layout

* `backlog.kdl` - `target <slug> tool=<binary>`, one line per demo that should exist.
* `<slug>/demo.kdl` - the manifest.
* `<slug>/.umbra/<tool>.guardfile.kdl` - the policy under demonstration.
* `.render/` - ignored. The workspace, `result.json`, the clip, and the review sheet.

## The manifest

```kdl
demo git-read-only {
    tool git
    state minted
    frame cols=80 rows=14
    setup {
        git-init
        file "notes.txt" "draft\n"
    }
    step "git push" exit=2 shows="is withheld"
}
```

* `state` - `minted`, `approved`, `bounced`, or `published`. Only a human moves it
  past `minted`.
* `frame` - the terminal in columns and rows. Verify fails any output that would
  wrap or scroll off.
* `setup` - `git-init`, `file <path> <body>`, and `guardfile <path>`, which copies
  the policy into view.
* `step` - one command, its exit code, and a substring its output must contain.

## Minting one

Run these from `demos/`.

1. `just next` prints the first queued slug, and `just new <slug>` scaffolds it.
2. Write the guardfile and the steps. Read [replacement](../guides/replacement.md) first.
3. `just verify <slug>` until it passes. It takes about 25 seconds.
4. `just render <slug>` records the clip in about 75 seconds. Open `last.png` and look.
5. `just sheet` writes `.render/index.html`, the page a reviewer approves from.

## What a step can reach

Every step runs with `HOME` inside the workspace, so no operator config or credential
reaches a demo. A step exits the way it would for a caller with no login, which is why
a demo shows refusals rather than remote reads.

## Recording

VHS is pinned to v0.11.0 as a `go tool` in `demos/go.mod`, because v0.12.0 exits 0
and writes no file
([charmbracelet/vhs#787](https://github.com/charmbracelet/vhs/issues/787)). Render
therefore probes the clip itself rather than trusting the exit code.

Off-camera lines type at 1ms, since at the on-camera speed the hidden env line alone
cost 23 seconds a render. Width and height round to even, because libx264 refuses an
odd dimension and vhs then leaves a zero-byte mp4 behind a clean exit.

`just gif <slug>` converts a rendered clip for markdown, which cannot embed mp4. It
stays out of render because encoding both formats doubled every render's cost.

A rerun renames the previous workspace aside before deleting it. Back-to-back runs
intermittently lost a `RemoveAll` race inside the previous run's `HOME`, and a rename
cannot.
