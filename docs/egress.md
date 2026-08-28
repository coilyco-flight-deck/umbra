# egress: the CONNECT proxy and its address guard

Part of [architecture.md](architecture.md). `http/egress` is the per-invocation
HTTP CONNECT proxy a consumer starts for the lifetime of one wrapped
subprocess. The child gets `HTTPS_PROXY`/`HTTP_PROXY` pointed at it, and every
tunnel it opens lands in the audit record.

## A name is not an address

The allowlist matches the hostname on the CONNECT line. DNS for a third-party
host is not under the operator's control, so an allowlisted name can resolve
inward: a CNAME to an internal host, a rebinding answer, or a vendor that
simply points a name at `127.0.0.1`. A string allowlist cannot see any of that.

So the proxy resolves the host itself, refuses the request when the answer is
internal, and **dials the address it just checked** rather than the name.
Checking and then re-resolving would leave the rebinding window open, which is
the whole point of fusing the two steps.

Refused ranges: loopback, unspecified, link-local unicast and multicast, and
private (RFC1918 plus IPv6 unique-local). The cloud metadata endpoint at
`169.254.169.254` is link-local, so it needs no special case.

**Any, not all.** A host is refused when *any* resolved address is internal. A
rebinding answer mixes a routable address with an internal one, so requiring
every answer to be internal would let the caller pick which one gets dialled.

**Not refused: carrier-grade NAT (`100.64.0.0/10`).** That is where the tailnet
lives, and reaching a tailnet service through a wrapped tool is ordinary use
here rather than an escape. Add it to `internalReason` if that stops being true.

## Ports

The allowlist names hosts, so on its own `CONNECT allowed-host:22` tunnels SSH
to an allowed host. `Proxy.AllowedPorts` bounds this and `New` installs
`{"443"}`, which is the only port a CONNECT proxy for HTTPS needs.

A consumer that must reach another port widens it explicitly:
`passthrough.WithEgressPorts("443", "8443")`. An empty `AllowedPorts` means any
port, which is what the package's own tests use against their random-port
server.

## Observe mode still refuses internal addresses

`ModeObserve` forwards every host and ignores the allowlist. It does **not**
lift the address guard. Observe is a statement about which hosts you have
enumerated yet, not a statement that reaching inward is acceptable, so the two
controls are deliberately independent.

## Refusal is distinguishable from unreachability

A guard refusal answers `403` and names itself; an address that is simply down
answers `502`. They were briefly collapsed into one response during
development, and the tests written against that could not tell the guard firing
from the host being unreachable, so they passed with the guard disabled. The
split exists so those tests can only pass for the right reason.

## Testing

`AllowLoopbackForTest` lifts the loopback case alone, because the package's own
tests tunnel to an `httptest` server on `127.0.0.1`. It is defined in
`export_test.go`, so no consumer can reach it, and it does not lift the private
or link-local cases.
