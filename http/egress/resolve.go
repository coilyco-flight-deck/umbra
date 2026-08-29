// Resolved-address guard. The allowlist matches a name, and a name is not an
// address: see docs/egress.md for why the two steps are fused here.
package egress

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"
)

// ErrAddressRefused marks a guard refusal, so the handler can answer it
// differently from an address that is merely unreachable.
var ErrAddressRefused = errors.New("egress: destination address refused")

// internalReason names why an address must not be dialled, or "" when routable.
// The cloud metadata endpoint is link-local, not a special case: docs/egress.md.
func internalReason(ip net.IP) string {
	switch {
	case ip.IsUnspecified():
		return "unspecified"
	case ip.IsLoopback():
		return "loopback"
	case ip.IsLinkLocalUnicast():
		return "link-local"
	case ip.IsLinkLocalMulticast(), ip.IsInterfaceLocalMulticast():
		return "link-local multicast"
	case ip.IsPrivate():
		return "private"
	}
	return ""
}

// resolveRoutable returns every address host may be dialled at, refusing the
// host when any single answer is internal. Any, not all: docs/egress.md.
func (p *Proxy) resolveRoutable(ctx context.Context, host string) ([]net.IP, error) {
	if ip := net.ParseIP(host); ip != nil {
		if reason := p.refuse(ip); reason != "" {
			return nil, fmt.Errorf("%w: %s address %s", ErrAddressRefused, reason, ip)
		}
		return []net.IP{ip}, nil
	}
	addrs, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("egress: resolve %s: %w", host, err)
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("egress: %s resolved to no addresses", host)
	}
	out := make([]net.IP, 0, len(addrs))
	for _, a := range addrs {
		if reason := p.refuse(a.IP); reason != "" {
			return nil, fmt.Errorf("%w: %s resolves to %s address %s", ErrAddressRefused, host, reason, a.IP)
		}
		out = append(out, a.IP)
	}
	return out, nil
}

// refuse reports why an address is barred, honouring the loopback exemption
// the package's own tests need to reach their httptest server.
func (p *Proxy) refuse(ip net.IP) string {
	reason := internalReason(ip)
	if reason == "loopback" && p.allowLoopback {
		return ""
	}
	return reason
}

// dialChecked dials the address it just checked rather than the name, so a
// second DNS answer cannot land between the check and the connection.
func (p *Proxy) dialChecked(ctx context.Context, host, port string) (net.Conn, error) {
	ips, err := p.resolveRoutable(ctx, host)
	if err != nil {
		return nil, err
	}
	d := net.Dialer{Timeout: 10 * time.Second}
	var lastErr error
	for _, ip := range ips {
		conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("egress: dial %s: %w", host, lastErr)
}

// portAllowed reports whether a CONNECT may target this port. An empty
// AllowedPorts means any port, which is not the default New installs.
func (p *Proxy) portAllowed(port string) bool {
	if len(p.AllowedPorts) == 0 {
		return true
	}
	for _, allowed := range p.AllowedPorts {
		if port == allowed {
			return true
		}
	}
	return false
}
