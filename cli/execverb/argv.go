package execverb

import "strings"

// valueFlags are the long flags whose value is a separate argv token.
// See docs/execverb-value-flags.md.
var valueFlags = map[string]bool{
	"--region": true, "--profile": true, "--output": true,
	"--endpoint-url": true, "--cli-read-timeout": true,
	"--cli-connect-timeout": true, "--color": true, "--ca-bundle": true,
	"--query": true,
}

// positionals returns argv with flags (and the values of known value-taking
// flags) removed, preserving order.
func positionals(argv []string) []string {
	out := make([]string, 0, len(argv))
	for i := 0; i < len(argv); i++ {
		tok := argv[i]
		if strings.HasPrefix(tok, "--") {
			if strings.IndexByte(tok, '=') >= 0 {
				continue // --flag=value, self-contained
			}
			if valueFlags[tok] && i+1 < len(argv) {
				i++ // consume the value
			}
			continue
		}
		if strings.HasPrefix(tok, "-") && tok != "-" {
			continue // short flag
		}
		out = append(out, tok)
	}
	return out
}

// globMatch reports whether s matches pattern, where `*` matches any run of
// characters (crossing `/` and `:`, unlike filepath.Match), the rest literal.
func globMatch(pattern, s string) bool {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == s // no wildcard: exact match
	}
	if parts[0] != "" {
		if !strings.HasPrefix(s, parts[0]) {
			return false
		}
		s = s[len(parts[0]):]
	}
	for _, mid := range parts[1 : len(parts)-1] {
		if mid == "" {
			continue
		}
		idx := strings.Index(s, mid)
		if idx < 0 {
			return false
		}
		s = s[idx+len(mid):]
	}
	last := parts[len(parts)-1]
	if last != "" {
		return strings.HasSuffix(s, last)
	}
	return true
}
