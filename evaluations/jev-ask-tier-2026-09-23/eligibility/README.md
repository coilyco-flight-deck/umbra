# Eligible n, corpus from umbra PR 388

2026-09-24, science seat, for `teable:coilyco-flight-deck/umbra#8119` on the corpus of
`teable:coilyco-flight-deck/umbra#8024` (PR 388). Counted from the corpus alone, before
any Jev call on it, so it is not a tally.

## Measured (`eligible-pr388.txt`)
* Accept 180 eligible, reject 170 eligible, 0 dropped in either class. Both clear the
  150 floor.
* The matcher reproduces the generator-verified `rule` on all 350 rows of the full
  guardfiles, with 0 disagreements. Only then is leave-one-rule-out counted.
* Negative control, not committed: two parent grants planted in a copy of the go
  guardfile (`can run "mod"`, `can run "work"`). The counter flagged all 13 affected
  rows and printed no count. It fails closed on any overlap, which is stricter than
  the drop-and-count rule in the preregistration.

## Scope of the matcher
Plain `can run` and `never run` word-paths only, matched by prefix over positional
words, as `never.go` does. Any other rule shape stops the script, so it cannot run
silently on a guardfile it does not understand. The original `ask-tier` regression set
uses deny-flag and deny-when, and is out of scope by design.
