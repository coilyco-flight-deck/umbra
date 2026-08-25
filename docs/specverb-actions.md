# complex actions

A **complex action** is a named composite verb authored inside a `wrap` block, orchestrating a bounded sequence of already-granted leaves with control flow. It is sugar over the allowlist, never an escape from it. See [specverb.md](specverb.md).

## The five invariants

1. **Granted-only.** An action may only target an op the same Guardfile grants via `can`. An ungranted target fails at `lock`/`build` time, not runtime.
2. **Bounded.** Every poll loop carries a mandatory `every` and `timeout`. No unbounded iteration exists in the grammar, which is what makes it reviewable.
3. **Per-call audit.** Each tick writes its own leaf `verb.Wrap` row; the action writes one envelope row.
4. **Dry-run is a plan.** `--dry-run` prints the call with bound params and the compiled `until`, firing nothing.
5. **One expression engine.** Conditions are JMESPath, the same engine `--query` uses, extended with native `$input` variables.

## Input defaulting

An `input` may carry `default <jmespath>`. When the operator omits it, the action fires the poll leaf **once as a pre-flight**, evaluates the expression against that response, and binds the result before the loop starts, so `ci-watch owner/repo` with no `--run` resolves to the latest run in the listing. The pre-flight hits only the poll leaf, introducing no new target and no new grant, and writes its own audit row like any tick.

## List inputs, and refusing before the write

An `input` may carry `array`, which makes its flag repeatable and projects the values as a JSON array rather than one string. Flags only: a positional list cannot be told from the arguments after it.

The element type is not declared here. It is read from the schema of the leaf field the arg binds, so `--labels 199 --labels 333` against a leaf declaring `items: {type: integer}` sends `[199, 333]` as numbers, and `--labels headless` against that same leaf is refused rather than sent. An array whose spec declares an empty `items` schema resolves each token on its own, the union encoding described in [specverb-request.md](specverb-request.md).

This is what lets a shadow refuse **before** a write. `required` on an action input is enforced while the inputs bind, which is before the request is assembled, so an action shadowing `create issue` can demand a label set and end the run with the hazard still absent. `fail-when` cannot do that job: it evaluates over the final response, so it reports a bad write rather than preventing one.

Two forms still carry scalars only, and both fail closed rather than flattening. A `collect` step pages a request built from string bindings, so binding an `array` input there is a build-time error. An exec step takes argv tokens, so a list reaching one is refused at the step.

## `collect`: auto-pagination

A `collect` action walks a granted list leaf page by page, appending every array response until a page returns fewer than the page size, then emits one accumulated array bound to `as`. It takes `page-param`, `limit-param`, and `default-limit`, and an optional `cache "<ttl>"` serving from the on-disk TTL cache. Granted-only, audited per page plus an envelope row, and dry-runnable like the rest.

## Mount actions: shadowing a generated leaf

An action authored with **two** header arguments (`action view issue` rather than `action <name>`) mounts at that leaf path and takes the place of the generated leaf, which is how a default verb grows behaviour: the operator keeps invoking `forgejo issue view` and it now resolves to a composite fetching the issue **and** its comment thread.

Three things follow. **It shadows**: the generated leaf is dropped from the CLI and describe surface, while the `can view issue` grant still resolves, so the action's own call reaches the op - the shadow replaces the CLI leaf, never the grant. **It combines**: a mount call-action renders every `as` binding together as one object rather than only the final call's response, and `--query` projects that combined shape. **It keeps the leaf's audit identity**: the envelope row is named for the shadowed path, so audit and metrics for that verb stay continuous while each inner call still writes its own row. A mount action may also be a `poll`; only the header arity differs.
