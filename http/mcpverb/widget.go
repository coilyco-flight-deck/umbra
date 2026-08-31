package mcpverb

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"forgejo.coilysiren.me/coilyco-flight-deck/umbra/http/opcore"
	"forgejo.coilysiren.me/coilyco-flight-deck/umbra/pkg/mcpapps"
	"forgejo.coilysiren.me/coilyco-flight-deck/umbra/pkg/mcpclient"
)

// WidgetGate is one granted tool's `widget` block resolved against the lock: the
// mcpapps.Policy a host consults before forwarding. See docs/mcpapps.md.
type WidgetGate struct {
	// tool is the instantiating tool whose view this gates, named in refusals.
	tool string

	// surface is what the view may see and call, deny by absence.
	surface []mcpclient.Tool

	// guards are the per-tool argument rules, keyed by upstream tool name.
	guards map[string]*opcore.MCPCall

	// reads are the compiled `can read` / `never read` sentences.
	reads []compiledRead

	// declared records whether the grant authored a `widget` block at all, so
	// the refusal names the missing block rather than a missing grant.
	declared bool
}

// The gate is what the host consults, so the seam is asserted at compile time
// rather than discovered when a view's first call arrives.
var _ mcpapps.Policy = (*WidgetGate)(nil)

// compiledRead is one read rule with its regex already built, so a match costs
// no compile and a malformed pattern was rejected at parse.
type compiledRead struct {
	deny bool
	re   *regexp.Regexp
	raw  string
}

// WidgetPolicy resolves the `widget` block of one granted tool into the policy
// the host consults, failing closed at build time. See docs/mcpapps.md.
func WidgetPolicy(cfg Config, tool string) (*WidgetGate, error) {
	if cfg.Guardfile == nil {
		return nil, fmt.Errorf("mcpverb: nil Guardfile")
	}
	byName := map[string]mcpclient.Tool{}
	names := make([]string, 0, len(cfg.Tools))
	for _, t := range cfg.Tools {
		byName[t.Name] = t
		names = append(names, t.Name)
	}
	grants, err := cfg.Guardfile.Granted(names)
	if err != nil {
		return nil, err
	}
	host, ok := grantFor(grants, tool)
	if !ok {
		return nil, fmt.Errorf("mcpverb: tool %q is not granted, so it has no view to gate (fail-closed)", tool)
	}
	if host.Widget == nil {
		return &WidgetGate{tool: tool, guards: map[string]*opcore.MCPCall{}}, nil
	}
	return buildGate(tool, host.Widget, byName)
}

// grantFor finds the resolved grant naming one tool.
func grantFor(grants []Grant, tool string) (Grant, bool) {
	for _, g := range grants {
		if g.Tool == tool {
			return g, true
		}
	}
	return Grant{}, false
}

// buildGate resolves a widget block against the lock, refusing a contradiction
// rather than picking a side.
func buildGate(tool string, w *Widget, byName map[string]mcpclient.Tool) (*WidgetGate, error) {
	denied := map[string]bool{}
	for _, g := range w.Grants {
		if g.IsDeny() {
			denied[g.Tool] = true
		}
	}
	gate := &WidgetGate{tool: tool, guards: map[string]*opcore.MCPCall{}, declared: true}
	seen := map[string]bool{}
	for _, g := range w.Grants {
		if g.IsDeny() {
			continue
		}
		if denied[g.Tool] {
			return nil, fmt.Errorf("mcpverb: widget of %s: tool %q is both granted and denied; drop one (fail-closed)", tool, g.Tool)
		}
		if seen[g.Tool] {
			return nil, fmt.Errorf("mcpverb: widget of %s: duplicate `can call %s` (fail-closed)", tool, g.Tool)
		}
		seen[g.Tool] = true
		called, held := byName[g.Tool]
		if !held {
			return nil, fmt.Errorf("mcpverb: widget of %s: view call %q is not in the lock; run `specgen lock` or fix the name (fail-closed)", tool, g.Tool)
		}
		if err := checkWidgetSelectors(tool, g, called); err != nil {
			return nil, err
		}
		gate.surface = append(gate.surface, called)
		gate.guards[g.Tool] = &opcore.MCPCall{Tool: g.Tool, Allow: g.Allow, Deny: g.Deny}
	}
	sort.Slice(gate.surface, func(i, j int) bool { return gate.surface[i].Name < gate.surface[j].Name })
	for _, r := range w.Reads {
		// Compiled at parse already, so this cannot fail; the error is kept
		// rather than dropped so a future pattern source cannot slip past.
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, fmt.Errorf("mcpverb: widget of %s: read regex %q does not compile: %w", tool, r.Pattern, err)
		}
		gate.reads = append(gate.reads, compiledRead{deny: r.IsDeny(), re: re, raw: r.Pattern})
	}
	return gate, nil
}

// checkWidgetSelectors refuses a view guard naming an argument the called tool
// does not have, against that tool's schema rather than the instantiating one's.
func checkWidgetSelectors(host string, g Grant, called mcpclient.Tool) error {
	fields, err := FieldsFor(called)
	if err != nil {
		return fmt.Errorf("mcpverb: widget of %s: %w", host, err)
	}
	if err := checkSelectors(g, fields); err != nil {
		return fmt.Errorf("mcpverb: widget of %s: %w", host, err)
	}
	return nil
}

// Tools is the surface this view may see and call.
func (w *WidgetGate) Tools() []mcpclient.Tool {
	out := make([]mcpclient.Tool, len(w.surface))
	copy(out, w.surface)
	return out
}

// CheckToolCall applies the view call's own allow and deny guards, through the
// same rule engine a CLI leaf uses so the two cannot drift.
func (w *WidgetGate) CheckToolCall(tool string, args map[string]any) error {
	call, ok := w.guards[tool]
	if !ok {
		if !w.declared {
			return fmt.Errorf("the grant for %s declares no `widget` block, so its view calls nothing", w.tool)
		}
		return fmt.Errorf("tool %q is not granted to the view of %s; add `can call %s` to its `widget` block", tool, w.tool, tool)
	}
	return opcore.CheckProxyRules(call.Allow, call.Deny, args)
}

// CheckResourceRead applies the read rules: a `never read` match refuses, and so
// does a URI no `can read` matches. Deny by absence, like every guard here.
func (w *WidgetGate) CheckResourceRead(uri string) error {
	for _, r := range w.reads {
		if r.deny && r.re.MatchString(uri) {
			return fmt.Errorf("resource %q is denied to the view of %s (never read %q)", uri, w.tool, r.raw)
		}
	}
	for _, r := range w.reads {
		if !r.deny && r.re.MatchString(uri) {
			return nil
		}
	}
	if !w.declared {
		return fmt.Errorf("the grant for %s declares no `widget` block, so its view reads nothing", w.tool)
	}
	return fmt.Errorf("resource %q is not readable by the view of %s; add `can read` to its `widget` block%s", uri, w.tool, allowedReads(w.reads))
}

// allowedReads names the patterns that would have permitted the read, so the
// refusal says what to widen rather than only that it refused.
func allowedReads(reads []compiledRead) string {
	var allowed []string
	for _, r := range reads {
		if !r.deny {
			allowed = append(allowed, fmt.Sprintf("%q", r.raw))
		}
	}
	if len(allowed) == 0 {
		return " (it declares none)"
	}
	return " (it permits " + strings.Join(allowed, ", ") + ")"
}
