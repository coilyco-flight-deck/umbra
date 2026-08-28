package egress_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"forgejo.coilysiren.me/coilyco-flight-deck/umbra/http/egress"
	"forgejo.coilysiren.me/coilyco-flight-deck/umbra/pkg/audit"
)

// startProxy starts a fresh proxy, returns its URL plus a cleanup that
// stops it and yields the collected rows.
func startProxy(t *testing.T, allow []string, mode egress.Mode) (proxyURL string, drain func() []audit.EgressRow) {
	t.Helper()
	p := egress.New(allow, mode)
	// The server under test is an httptest one on 127.0.0.1 at a random port,
	// which is exactly what the resolved-address and port guards refuse.
	p.AllowLoopbackForTest()
	p.AllowedPorts = nil
	u, err := p.Start(context.Background())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = p.Stop() })
	return u, p.Stop
}

// dialThroughProxy issues a CONNECT to the proxy and returns the resulting
// duplex stream (already past the 200 line).
func dialThroughProxy(t *testing.T, proxyURL, hostport string) (net.Conn, error) {
	t.Helper()
	u, err := url.Parse(proxyURL)
	if err != nil {
		return nil, err
	}
	c, err := net.DialTimeout("tcp", u.Host, 2*time.Second)
	if err != nil {
		return nil, err
	}
	req := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", hostport, hostport)
	if _, err := c.Write([]byte(req)); err != nil {
		_ = c.Close()
		return nil, err
	}
	// Read status line.
	buf := make([]byte, 1024)
	n, err := c.Read(buf)
	if err != nil {
		_ = c.Close()
		return nil, err
	}
	resp := string(buf[:n])
	if !strings.HasPrefix(resp, "HTTP/1.1 200") {
		_ = c.Close()
		return nil, fmt.Errorf("non-200 from proxy: %q", strings.TrimSpace(resp))
	}
	return c, nil
}

// newHTTPSServer starts a TLS test server and returns its host:port plus a
// CA cert pool that trusts it. The proxy itself does no TLS, so the client
func newHTTPSServer(t *testing.T, body string) (hostport string, pool *x509.CertPool) {
	t.Helper()
	s := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(s.Close)
	u, err := url.Parse(s.URL)
	if err != nil {
		t.Fatalf("parse server URL: %v", err)
	}
	pool = x509.NewCertPool()
	pool.AddCert(s.Certificate())
	return u.Host, pool
}

func TestProxy_AllowedHostForwards(t *testing.T) {
	hostport, pool := newHTTPSServer(t, "hello-egress")
	host, _, _ := net.SplitHostPort(hostport)

	proxyURL, drain := startProxy(t, []string{host}, egress.ModeEnforce)

	tunnel, err := dialThroughProxy(t, proxyURL, hostport)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	tlsConn := tls.Client(tunnel, &tls.Config{
		ServerName: host,
		RootCAs:    pool,
		MinVersion: tls.VersionTLS12,
	})
	if err := tlsConn.Handshake(); err != nil {
		t.Fatalf("tls handshake: %v", err)
	}
	req := fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nConnection: close\r\n\r\n", host)
	if _, err := tlsConn.Write([]byte(req)); err != nil {
		t.Fatalf("write: %v", err)
	}
	body, err := io.ReadAll(tlsConn)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(body), "hello-egress") {
		t.Errorf("body = %q, want it to contain hello-egress", string(body))
	}
	_ = tlsConn.Close()

	rows := drain()
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1: %+v", len(rows), rows)
	}
	if rows[0].Decision != audit.EgressAllow {
		t.Errorf("decision = %q, want allow", rows[0].Decision)
	}
	if rows[0].Host != host {
		t.Errorf("host = %q, want %q", rows[0].Host, host)
	}
	if rows[0].BytesUp == 0 || rows[0].BytesDown == 0 {
		t.Errorf("bytes up=%d down=%d, want both >0", rows[0].BytesUp, rows[0].BytesDown)
	}
}

func TestProxy_DeniedHostReturns403(t *testing.T) {
	proxyURL, drain := startProxy(t, []string{"only.allowed.example"}, egress.ModeEnforce)

	u, _ := url.Parse(proxyURL)
	c, err := net.DialTimeout("tcp", u.Host, 2*time.Second)
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer c.Close()
	req := "CONNECT denied.example:443 HTTP/1.1\r\nHost: denied.example:443\r\n\r\n"
	if _, err := c.Write([]byte(req)); err != nil {
		t.Fatalf("write: %v", err)
	}
	buf := make([]byte, 1024)
	n, err := c.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	resp := string(buf[:n])
	if !strings.HasPrefix(resp, "HTTP/1.1 403") {
		t.Errorf("status line = %q, want 403", strings.SplitN(resp, "\r\n", 2)[0])
	}

	rows := drain()
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1: %+v", len(rows), rows)
	}
	if rows[0].Decision != audit.EgressDeny {
		t.Errorf("decision = %q, want deny", rows[0].Decision)
	}
	if rows[0].Host != "denied.example" {
		t.Errorf("host = %q, want denied.example", rows[0].Host)
	}
}

func TestProxy_ObserveModeAllowsEverything(t *testing.T) {
	hostport, pool := newHTTPSServer(t, "ok")
	host, _, _ := net.SplitHostPort(hostport)

	// Empty allowlist + ModeObserve must still forward.
	proxyURL, drain := startProxy(t, nil, egress.ModeObserve)

	tunnel, err := dialThroughProxy(t, proxyURL, hostport)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	tlsConn := tls.Client(tunnel, &tls.Config{
		ServerName: host,
		RootCAs:    pool,
		MinVersion: tls.VersionTLS12,
	})
	if err := tlsConn.Handshake(); err != nil {
		t.Fatalf("tls handshake: %v", err)
	}
	_, _ = tlsConn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n"))
	_, _ = io.ReadAll(tlsConn)
	_ = tlsConn.Close()

	rows := drain()
	if len(rows) != 1 || rows[0].Decision != audit.EgressAllow {
		t.Errorf("rows = %+v, want one allow row", rows)
	}
}

func TestProxy_AggregatesByHost(t *testing.T) {
	hostport, pool := newHTTPSServer(t, "x")
	host, _, _ := net.SplitHostPort(hostport)

	proxyURL, drain := startProxy(t, []string{host}, egress.ModeEnforce)

	for i := 0; i < 3; i++ {
		tunnel, err := dialThroughProxy(t, proxyURL, hostport)
		if err != nil {
			t.Fatalf("dial %d: %v", i, err)
		}
		tlsConn := tls.Client(tunnel, &tls.Config{
			ServerName: host,
			RootCAs:    pool,
			MinVersion: tls.VersionTLS12,
		})
		if err := tlsConn.Handshake(); err != nil {
			t.Fatalf("tls %d: %v", i, err)
		}
		_, _ = tlsConn.Write([]byte("GET / HTTP/1.1\r\nHost: x\r\nConnection: close\r\n\r\n"))
		_, _ = io.ReadAll(tlsConn)
		_ = tlsConn.Close()
	}
	rows := drain()
	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1 aggregated row: %+v", len(rows), rows)
	}
}

func TestProxy_RejectsNonConnect(t *testing.T) {
	proxyURL, _ := startProxy(t, nil, egress.ModeObserve)
	resp, err := http.Get(proxyURL + "/anything") //nolint:noctx
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", resp.StatusCode)
	}
}

// startGuardedProxy starts a proxy with the resolved-address and port guards
// fully armed, which is the shape a consumer actually gets.
func startGuardedProxy(t *testing.T, allow []string) string {
	t.Helper()
	p := egress.New(allow, egress.ModeEnforce)
	u, err := p.Start(context.Background())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = p.Stop() })
	return u
}

func TestProxy_AllowlistedNameResolvingInwardIsRefused(t *testing.T) {
	// The whole point of the guard: "localhost" passes a string allowlist and
	// resolves to loopback, which a name-only check cannot see.
	proxyURL := startGuardedProxy(t, []string{"localhost"})

	_, err := dialThroughProxy(t, proxyURL, "localhost:443")
	requireRefused(t, err, "allowlisted host resolving to loopback")
}

func TestProxy_AllowlistedInternalLiteralsAreRefused(t *testing.T) {
	for _, addr := range []string{
		"169.254.169.254", // cloud metadata, link-local
		"10.0.0.1",        // RFC1918
		"192.168.1.1",     // RFC1918
		"0.0.0.0",         // unspecified
	} {
		t.Run(addr, func(t *testing.T) {
			proxyURL := startGuardedProxy(t, []string{addr})
			_, err := dialThroughProxy(t, proxyURL, addr+":443")
			requireRefused(t, err, "allowlisted internal literal "+addr)
		})
	}
}

func TestProxy_NonHTTPSPortOnAnAllowedHostIsRefused(t *testing.T) {
	// The allowlist names hosts, so without the port guard CONNECT to :22 on
	// an allowed host tunnels SSH.
	hostport, _ := newHTTPSServer(t, "unreachable")
	host, _, _ := net.SplitHostPort(hostport)
	proxyURL := startGuardedProxy(t, []string{host})

	_, err := dialThroughProxy(t, proxyURL, host+":22")
	requireDenied(t, err, "CONNECT to port 22 on an allowed host")
}

func TestProxy_ObserveModeStillRefusesInternalAddresses(t *testing.T) {
	// Observe governs the allowlist, not safety, so it forwards any host but
	// still must not reach inward.
	p := egress.New(nil, egress.ModeObserve)
	p.AllowedPorts = nil
	u, err := p.Start(context.Background())
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = p.Stop() })

	_, err = dialThroughProxy(t, u, "169.254.169.254:80")
	requireRefused(t, err, "observe mode reaching the metadata address")
}

func TestInternalReasonClassifiesTheInterestingAddresses(t *testing.T) {
	internal := map[string]string{
		"169.254.169.254":  "link-local",
		"127.0.0.1":        "loopback",
		"::1":              "loopback",
		"::ffff:127.0.0.1": "loopback",
		"10.1.2.3":         "private",
		"172.16.0.1":       "private",
		"fd00::1":          "private",
		"0.0.0.0":          "unspecified",
	}
	for addr, want := range internal {
		if got := egress.InternalReasonForTest(addr); got != want {
			t.Errorf("%s: reason %q, want %q", addr, got, want)
		}
	}
	for _, addr := range []string{"93.184.216.34", "1.1.1.1", "2606:2800:220:1:248:1893:25c8:1946"} {
		if got := egress.InternalReasonForTest(addr); got != "" {
			t.Errorf("%s: routable address refused as %q", addr, got)
		}
	}
}

// requireRefused fails unless the proxy answered with the address guard's own
// refusal. An unreachable address answers 502, so this cannot pass by accident.
func requireRefused(t *testing.T, err error, what string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s was tunnelled", what)
	}
	if !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "destination address refused") {
		t.Fatalf("%s: want the address guard's 403 refusal, got %v", what, err)
	}
}

// requireDenied fails unless the proxy answered with an allowlist denial.
func requireDenied(t *testing.T, err error, what string) {
	t.Helper()
	if err == nil {
		t.Fatalf("%s was tunnelled", what)
	}
	if !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "denied by allowlist") {
		t.Fatalf("%s: want a 403 allowlist denial, got %v", what, err)
	}
}
