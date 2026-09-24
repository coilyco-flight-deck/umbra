# Negative controls

`umbra controls` checks that every refusal a guardfile states actually refuses. For each `never run` rule and `withhold` stub in the exec dialect, and each `never` or `cannot` grant in the spec dialect, it does two things:

1. **Invoke the refused path as a caller would.** The rule holds only if the call exits `policy_denied` with that rule's own refusal text and reaches nothing. Exit 2 alone is not enough, because a call refused for some other reason also exits 2, and that is how a doc-only `never run` looked like a denial while denying nothing (`teable:coilyco-flight-deck/umbra#8120`).
2. **Invoke it again with that one rule removed.** The outcome has to change. A rule whose removal changes nothing is not load-bearing, and the run names it rather than counting it as a pass.

The run prints one line per rule and a final count, and exits non-zero when any rule does not hold. A member with no rules is printed too, so the count covers everything that was looked at.

```sh
umbra --project-root .umbra/guardfiles/aosguard controls
```

## What it observes, and what it does not

Refusal is judged from the observed exit and error text of a real invocation. It is never judged from which commands happen to be mounted, since an unmounted path is refused anyway. And it is never judged from the audit log, which has no row for these calls (`teable:coilyco-flight-deck/umbra#8121`).

Every invocation goes through the same build path the generated binary uses: `execverb.Mount` for a wrap, and `BuildReplacement` plus the `default-allow` fallback for a replacement. The seams are replaced so nothing leaves the process:

* the exec runner records that the binary would have run
* `shell` selectors fail closed
* the HTTP transport records the request and answers an empty `200`
* every value provider answers a placeholder

No binary, network endpoint, or secret store is touched.

urfave adds its help command once `Run` starts, and that command exits through the package-level `cli.OsExiter`. So `negcontrol.Run` swaps that global for the length of one invocation. That makes the check unsafe to run concurrently in one process.

## Where it runs

It is authored here as a verb any consumer can run against its own project roots. Wiring it into a consumer's checks belongs to that consumer. MCP-dialect members carry no refusal rule, so they are skipped.
