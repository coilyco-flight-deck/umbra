package egress

import "net"

// netParseIP is a thin alias so export_test.go needs no build-tag gymnastics.
func netParseIP(s string) net.IP { return net.ParseIP(s) }

// AllowLoopbackForTest lifts only the loopback half of the resolved-address
// guard, so the package's own tests can reach their httptest server.
func (p *Proxy) AllowLoopbackForTest() { p.allowLoopback = true }

// InternalReasonForTest exposes the address classifier so the metadata-endpoint
// and IPv4-mapped cases can be asserted directly.
func InternalReasonForTest(ip string) string { return internalReason(netParseIP(ip)) }
