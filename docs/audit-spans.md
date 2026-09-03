# audit records as spans

umbra's append-only audit log projects onto tracing spans, so a refusal is a
telemetry signal rather than a log artifact and an operator can see prevented
actions as a **population** rather than one at a time (umbra#6817).

## The claim this is built against, and the one it is not

The wide claim is occupied one layer up. Harness ships Agent Trace, and
Coralogix and Oracle both describe governance signals recording which policies
blocked an action. They instrument the model and policy layer.

What survives is narrower, and is the only claim this supports: **umbra
instruments the process boundary and already publishes an exit-code taxonomy
separating refusal from failure**, which none of those does. Do not restate the
wider claim.

## umbra ships no OTel SDK

`Record.SpanOf()` returns an `audit.Span`: a name, a start time, a duration, an
error flag, and flat attributes. No SDK type appears in the signature.

This is the same line `pkg/valuesource` draws. umbra is the base of the stack
and a dependency here reaches every generated binary and every consumer's
supply chain, so umbra states the shape and the consumer wires the exporter:

```go
w := audit.NewWriter(path)
w.Sinks = []audit.Sink{audit.SinkFunc(func(s audit.Span) {
    // consumer's tracer, consumer's SDK version, consumer's sampling
})}
```

Sinks run **after** the durable JSONL write and cannot return an error.
Telemetry never fails an audited invocation, and the log never depends on a
collector being reachable.

## Refusal is not an error

`OutcomeFor` classifies the exit code into four outcomes, and this is the whole
point of the projection rather than a detail of it:

| exit code | outcome | span error |
| --- | --- | --- |
| `Success` | `ok` | no |
| `PolicyDenied` | `refused` | **no** |
| `Internal` | `internal` | yes |
| everything else | `failed` | yes |

A refusal is a **successful boundary**. Marking its span an error would bury it
in the same bucket as a broken upstream, which is exactly the confusion the
exit-code taxonomy exists to prevent. `umbra.refused` is a boolean attribute so
a query selects the population directly, without parsing an error string.

## Attributes

`umbra.verb`, `umbra.decision`, `umbra.outcome`, `umbra.exit_code`,
`umbra.kind`, `umbra.refused`, and, when set, `umbra.session_id`,
`umbra.repo_root`, `umbra.cache`, `umbra.policy_skipped`,
`umbra.egress_host_count`. Unset fields are omitted rather than emitted empty,
so a backend does not index a column of blanks.

The keys are namespaced so a collector can select umbra's rows without matching
on a verb name, and they are **stable**: a consumer's dashboard queries them.

The span is built from the record *after* redaction, so a sink can never carry
a value the JSONL would not.

## What is not here

The operator dashboard umbra#6817 also asks for. It renders in a hosted
observability surface against a consumer's exporter, so it is neither umbra's
code nor umbra's call.
