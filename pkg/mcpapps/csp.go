package mcpapps

import "strings"

// CSPSources are the domains a view declared, one field per directive family
// the spec's `csp` object carries. See docs/mcpapps.md.
type CSPSources struct {
	// Resource feeds script-src, style-src, img-src, font-src, and media-src.
	Resource []string

	// Connect feeds connect-src and is what `can connect` declares. The other
	// three families have no guardfile verb yet.
	Connect []string

	// Frame feeds frame-src, and BaseURI feeds base-uri.
	Frame   []string
	BaseURI []string

	// Report names a collector path for violation reports. It is umbra's
	// addition rather than the spec's, and it grants nothing.
	Report string
}

// Declared reports whether the view named any domain. None of them selects the
// spec's omitted-`ui.csp` case, which is the tighter of the two.
func (s CSPSources) Declared() bool {
	return len(s.Resource) > 0 || len(s.Connect) > 0 ||
		len(s.Frame) > 0 || len(s.BaseURI) > 0
}

// omittedCSP is what the spec requires verbatim when `ui.csp` is absent.
var omittedCSP = []string{
	"default-src 'none'",
	"script-src 'self' 'unsafe-inline'",
	"style-src 'self' 'unsafe-inline'",
	"img-src 'self' data:",
	"media-src 'self' data:",
	"object-src 'none'",
	"connect-src 'none'",
}

// CSP builds the policy for one view. An undeclared family falls back to the
// spec's own default, so a missing declaration refuses rather than opens.
func CSP(s CSPSources) string {
	var directives []string
	if !s.Declared() {
		directives = append(directives, omittedCSP...)
	} else {
		resource := join(s.Resource)
		directives = []string{
			"default-src 'none'",
			"script-src 'self' 'unsafe-inline'" + resource,
			"style-src 'self' 'unsafe-inline'" + resource,
			"connect-src 'self'" + join(s.Connect),
			"img-src 'self' data:" + resource,
			"font-src 'self'" + resource,
			"media-src 'self' data:" + resource,
			"frame-src " + orNone(s.Frame),
			"object-src 'none'",
			"base-uri " + orSelf(s.BaseURI),
		}
	}
	if report := strings.TrimSpace(s.Report); report != "" {
		directives = append(directives, "report-uri "+report)
	}
	return strings.Join(directives, "; ") + ";"
}

func join(sources []string) string {
	if len(sources) == 0 {
		return ""
	}
	return " " + strings.Join(sources, " ")
}

func orNone(sources []string) string {
	if len(sources) == 0 {
		return "'none'"
	}
	return strings.Join(sources, " ")
}

func orSelf(sources []string) string {
	if len(sources) == 0 {
		return "'self'"
	}
	return strings.Join(sources, " ")
}
