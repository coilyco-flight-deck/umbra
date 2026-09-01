package mcpapps

import (
	"strings"
	"testing"
)

// The spec's omitted-`ui.csp` block, reproduced from specification/draft/apps.mdx
// so a drift in either direction fails here rather than in a browser.
func TestCSPWithNothingDeclaredIsTheSpecDefault(t *testing.T) {
	got := CSP(CSPSources{})
	want := "default-src 'none'; script-src 'self' 'unsafe-inline'; " +
		"style-src 'self' 'unsafe-inline'; img-src 'self' data:; " +
		"media-src 'self' data:; object-src 'none'; connect-src 'none';"
	if got != want {
		t.Fatalf("default policy drifted:\n got %q\nwant %q", got, want)
	}
}

// The omitted case is tighter than the constructed one, which is the property
// that makes an absent declaration a refusal. connect-src is where it shows.
func TestTheUndeclaredPolicyIsTighterThanTheDeclaredOne(t *testing.T) {
	if !strings.Contains(CSP(CSPSources{}), "connect-src 'none'") {
		t.Fatal("an undeclared view should reach nothing")
	}
	declared := CSP(CSPSources{Connect: []string{"https://api.example.com"}})
	if !strings.Contains(declared, "connect-src 'self' https://api.example.com;") {
		t.Fatalf("a declared origin should reach connect-src: %q", declared)
	}
	if strings.Contains(declared, "connect-src 'none'") {
		t.Fatalf("the two shapes should not both apply: %q", declared)
	}
}

// Declaring one family must not loosen another. A view that names a connect
// origin still gets 'none' for frames and 'self' for its base URI.
func TestDeclaringOneFamilyLeavesTheOthersAtTheirFloor(t *testing.T) {
	got := CSP(CSPSources{Connect: []string{"https://api.example.com"}})
	for _, want := range []string{
		"frame-src 'none'", "base-uri 'self'", "object-src 'none'",
		"script-src 'self' 'unsafe-inline';", "img-src 'self' data:;",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestResourceDomainsReachEveryFamilyTheSpecGivesThem(t *testing.T) {
	got := CSP(CSPSources{Resource: []string{"https://cdn.example.com"}})
	for _, want := range []string{
		"script-src 'self' 'unsafe-inline' https://cdn.example.com",
		"style-src 'self' 'unsafe-inline' https://cdn.example.com",
		"img-src 'self' data: https://cdn.example.com",
		"font-src 'self' https://cdn.example.com",
		"media-src 'self' data: https://cdn.example.com",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "connect-src 'self' https://cdn.example.com") {
		t.Fatal("a resource domain must not become a connect domain")
	}
}

// The report collector grants nothing, so it must attach to either shape
// without changing a directive.
func TestTheReportCollectorAddsNoPermission(t *testing.T) {
	bare := CSP(CSPSources{})
	reporting := CSP(CSPSources{Report: "/csp-report"})
	if !strings.HasPrefix(reporting, bare[:len(bare)-1]) {
		t.Fatalf("reporting changed the policy:\n %q\n %q", reporting, bare)
	}
	if !strings.HasSuffix(reporting, "report-uri /csp-report;") {
		t.Fatalf("the collector is not named: %q", reporting)
	}
}
