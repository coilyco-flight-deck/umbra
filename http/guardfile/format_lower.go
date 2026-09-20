// format_lower: the YAML and TOML schema, as tables from key to lowering. A key
// absent from a table is an error, so a misspelling cannot silently drop a rule,
// and the `kdl` key carries any node the schema has no key for.

package guardfile

import (
	"fmt"
	"sort"
	"strings"

	kdl "github.com/calico32/kdl-go"
)

// fieldFn lowers one key's value, where names the key for errors.
type fieldFn func(w *writer, v any, where string) error

func nested(where, key string) string {
	if where == "" {
		return key
	}
	return where + "." + key
}

// checkKeys fails closed on a key the table does not name.
func checkKeys(m *omap, where string, table map[string]fieldFn) error {
	for _, k := range m.keys {
		if _, ok := table[k]; !ok {
			loc := where
			if loc == "" {
				loc = "document"
			}
			return fmt.Errorf("%s: unknown key %q (fail-closed, want %s, or `kdl` for a node with no key)", loc, k, known(table))
		}
	}
	return nil
}

// lowerFields lowers each key of m in author order through table.
func lowerFields(w *writer, m *omap, where string, table map[string]fieldFn) error {
	if err := checkKeys(m, where, table); err != nil {
		return err
	}
	for _, k := range m.keys {
		if err := table[k](w, m.vals[k], nested(where, k)); err != nil {
			return err
		}
	}
	return nil
}

func known(table map[string]fieldFn) string {
	names := make([]string, 0, len(table))
	for k := range table {
		names = append(names, k)
	}
	sort.Strings(names)
	return strings.Join(names, ", ")
}

func skip(*writer, any, string) error { return nil }

func requireKey(m *omap, key, where string) (any, error) {
	v, ok := m.get(key)
	if !ok {
		return nil, fmt.Errorf("%s: missing required key %q", where, key)
	}
	return v, nil
}

func asMap(v any, where string) (*omap, error) {
	m, ok := v.(*omap)
	if !ok {
		return nil, fmt.Errorf("%s: want a mapping, got %s", where, describe(v))
	}
	return m, nil
}

func asList(v any, where string) ([]any, error) {
	l, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%s: want a list, got %s", where, describe(v))
	}
	return l, nil
}

func asString(v any, where string) (string, error) {
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%s: want a string, got %s", where, describe(v))
	}
	return s, nil
}

// asStrings takes one string or a list of strings.
func asStrings(v any, where string) ([]any, error) {
	if s, ok := v.(string); ok {
		return []any{s}, nil
	}
	l, err := asList(v, where)
	if err != nil {
		return nil, err
	}
	for i, e := range l {
		if _, ok := e.(string); !ok {
			return nil, fmt.Errorf("%s[%d]: want a string, got %s", where, i, describe(e))
		}
	}
	return l, nil
}

func asBool(v any, where string) (bool, error) {
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("%s: want true or false, got %s", where, describe(v))
	}
	return b, nil
}

// each runs fn over a list of mappings, naming the entry in errors.
func each(v any, where string, fn func(m *omap, at string) error) error {
	items, err := asList(v, where)
	if err != nil {
		return err
	}
	for i, it := range items {
		at := fmt.Sprintf("%s[%d]", where, i)
		m, err := asMap(it, at)
		if err != nil {
			return err
		}
		if err := fn(m, at); err != nil {
			return err
		}
	}
	return nil
}

// rawKDL validates a `kdl` snippet standalone, so unbalanced braces cannot break
// out of the block that carries it, then writes it in place.
func rawKDL(w *writer, v any, where string) error {
	s, err := asString(v, where)
	if err != nil {
		return err
	}
	if _, perr := kdl.ParseString(s); perr != nil {
		return fmt.Errorf("%s: the raw KDL does not parse on its own: %w", where, perr)
	}
	for _, l := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		if strings.TrimSpace(l) != "" {
			w.line(strings.TrimSpace(l))
		}
	}
	return nil
}

// simple lowers a key to a node with one string argument, named for the key
// unless name says otherwise.
func simple(name string) fieldFn {
	return func(w *writer, v any, where string) error {
		s, err := asString(v, where)
		if err != nil {
			return err
		}
		return w.leaf(name, []any{s}, nil)
	}
}

// names lowers a key to a node carrying a list of strings as its arguments.
func names(name string) fieldFn {
	return func(w *writer, v any, where string) error {
		list, err := asStrings(v, where)
		if err != nil {
			return err
		}
		return w.leaf(name, list, nil)
	}
}

// props lowers a mapping to a node carrying its keys as properties. The keys are
// data (a JSON field, a `set` target), so they are never renamed.
func props(name string) fieldFn {
	return func(w *writer, v any, where string) error {
		m, err := asMap(v, where)
		if err != nil {
			return err
		}
		out := make([]kv, 0, len(m.keys))
		for _, k := range m.keys {
			out = append(out, kv{k, m.vals[k]})
		}
		return w.leaf(name, nil, out)
	}
}

// document ------------------------------------------------------------------

var documentFields map[string]fieldFn

func init() {
	documentFields = map[string]fieldFn{
		"description":           simple("description"),
		"instructions":          lowerInstructions,
		"wrap":                  lowerWrap,
		"withhold":              lowerWithhold,
		"reject_empty_argument": lowerRejectEmpty,
		"kdl":                   rawKDL,
	}
}

func lowerDocument(tree any) ([]byte, error) {
	root, err := asMap(tree, "document")
	if err != nil {
		return nil, err
	}
	w := &writer{}
	if err := lowerFields(w, root, "", documentFields); err != nil {
		return nil, err
	}
	return []byte(w.b.String()), nil
}

func lowerInstructions(w *writer, v any, where string) error {
	lines, err := asStrings(v, where)
	if err != nil {
		return err
	}
	closeBlock, err := w.node("instructions", nil, nil, true)
	if err != nil {
		return err
	}
	for _, l := range lines {
		if err := w.leaf("text", []any{l}, nil); err != nil {
			return err
		}
	}
	closeBlock()
	return nil
}

var withholdFields = map[string]fieldFn{"tool": skip, "reason": skip, "alternative": skip}

func lowerWithhold(w *writer, v any, where string) error {
	return each(v, where, func(m *omap, at string) error {
		if err := checkKeys(m, at, withholdFields); err != nil {
			return err
		}
		tool, err := requireKey(m, "tool", at)
		if err != nil {
			return err
		}
		words, err := asStrings(tool, at+".tool")
		if err != nil {
			return err
		}
		_, hasReason := m.get("reason")
		_, hasAlt := m.get("alternative")
		closeBlock, err := w.node("withhold", words, nil, hasReason || hasAlt)
		if err != nil {
			return err
		}
		for _, k := range []string{"reason", "alternative"} {
			if val, ok := m.get(k); ok {
				if err := simple(k)(w, val, at+"."+k); err != nil {
					return err
				}
			}
		}
		closeBlock()
		return nil
	})
}

func lowerRejectEmpty(w *writer, v any, where string) error {
	return each(v, where, func(m *omap, at string) error {
		if err := checkKeys(m, at, map[string]fieldFn{"tool": skip, "field": skip}); err != nil {
			return err
		}
		tool, err := requireKey(m, "tool", at)
		if err != nil {
			return err
		}
		field, err := requireKey(m, "field", at)
		if err != nil {
			return err
		}
		return w.leaf("reject-empty-argument", []any{tool}, []kv{{"field", field}})
	})
}

// wrap ----------------------------------------------------------------------

var wrapFields map[string]fieldFn

func init() {
	wrapFields = map[string]fieldFn{
		"command":              skip,
		"inherit":              lowerInherit,
		"spec":                 simple("spec"),
		"base_url":             lowerBaseURL,
		"auth":                 lowerAuth,
		"restrict":             lowerRestrict,
		"allow_metacharacters": names("allow-metacharacters"),
		"can":                  lowerGrants("can"),
		"cannot":               lowerGrants("cannot"),
		"never":                lowerGrants("never"),
		"override":             lowerGrants("override"),
		"provider":             lowerProviders,
		"kdl":                  rawKDL,
	}
}

func lowerWrap(w *writer, v any, where string) error {
	m, err := asMap(v, where)
	if err != nil {
		return err
	}
	cmd, err := requireKey(m, "command", where)
	if err != nil {
		return err
	}
	words, err := asStrings(cmd, where+".command")
	if err != nil {
		return err
	}
	closeBlock, err := w.node("wrap", words, nil, true)
	if err != nil {
		return err
	}
	if err := lowerFields(w, m, where, wrapFields); err != nil {
		return err
	}
	closeBlock()
	return nil
}

// lowerInherit writes one `inherit` node per path, since the loader wants exactly
// one path per directive. They keep author order and must lead the wrap body.
func lowerInherit(w *writer, v any, where string) error {
	paths, err := asStrings(v, where)
	if err != nil {
		return err
	}
	for _, p := range paths {
		if err := w.leaf("inherit", []any{p}, nil); err != nil {
			return err
		}
	}
	return nil
}

func lowerBaseURL(w *writer, v any, where string) error {
	if s, ok := v.(string); ok {
		return w.leaf("base-url", []any{s}, nil)
	}
	m, err := asMap(v, where)
	if err != nil {
		return err
	}
	src, err := requireKey(m, "value", where)
	if err != nil {
		return err
	}
	if err := checkKeys(m, where, map[string]fieldFn{"value": skip}); err != nil {
		return err
	}
	closeBlock, err := w.node("base-url", nil, nil, true)
	if err != nil {
		return err
	}
	if err := lowerValue(w, src, where+".value"); err != nil {
		return err
	}
	closeBlock()
	return nil
}

// lowerValue writes a value source: `{env: NAME}` is one source, and a list of
// them is an ordered fallback chain where the first that yields wins.
func lowerValue(w *writer, v any, where string) error {
	list, chain := v.([]any)
	if !chain {
		provider, addr, err := oneSource(v, where)
		if err != nil {
			return err
		}
		return w.leaf("value", []any{provider, addr}, nil)
	}
	closeBlock, err := w.node("value", nil, nil, true)
	if err != nil {
		return err
	}
	for i, it := range list {
		provider, addr, err := oneSource(it, fmt.Sprintf("%s[%d]", where, i))
		if err != nil {
			return err
		}
		if err := w.leaf(provider, []any{addr}, nil); err != nil {
			return err
		}
	}
	closeBlock()
	return nil
}

func oneSource(v any, where string) (provider, addr string, err error) {
	m, err := asMap(v, where)
	if err != nil {
		return "", "", err
	}
	if len(m.keys) != 1 {
		return "", "", fmt.Errorf("%s: a value source names exactly one provider, e.g. {env: NAME}", where)
	}
	addr, err = asString(m.vals[m.keys[0]], where+"."+m.keys[0])
	return m.keys[0], addr, err
}

var authFields map[string]fieldFn

func init() {
	authFields = map[string]fieldFn{
		"scheme": skip,
		"header": simple("header"),
		"prefix": simple("prefix"),
		"value":  lowerValue,
		"params": lowerAuthParams,
	}
}

func lowerAuth(w *writer, v any, where string) error {
	m, err := asMap(v, where)
	if err != nil {
		return err
	}
	scheme, err := requireKey(m, "scheme", where)
	if err != nil {
		return err
	}
	name, err := asString(scheme, where+".scheme")
	if err != nil {
		return err
	}
	closeBlock, err := w.node("auth", []any{name}, nil, len(m.keys) > 1)
	if err != nil {
		return err
	}
	if err := lowerFields(w, m, where, authFields); err != nil {
		return err
	}
	closeBlock()
	return nil
}

func lowerAuthParams(w *writer, v any, where string) error {
	return each(v, where, func(m *omap, at string) error {
		name, err := requireKey(m, "name", at)
		if err != nil {
			return err
		}
		src, err := requireKey(m, "value", at)
		if err != nil {
			return err
		}
		if err := checkKeys(m, at, map[string]fieldFn{"name": skip, "value": skip}); err != nil {
			return err
		}
		closeBlock, err := w.node("param", []any{name}, nil, true)
		if err != nil {
			return err
		}
		if err := lowerValue(w, src, at+".value"); err != nil {
			return err
		}
		closeBlock()
		return nil
	})
}

func lowerRestrict(w *writer, v any, where string) error {
	return each(v, where, func(m *omap, at string) error {
		param, err := requireKey(m, "param", at)
		if err != nil {
			return err
		}
		globs, err := requireKey(m, "matches", at)
		if err != nil {
			return err
		}
		if err := checkKeys(m, at, map[string]fieldFn{"param": skip, "matches": skip}); err != nil {
			return err
		}
		list, err := asStrings(globs, at+".matches")
		if err != nil {
			return err
		}
		return w.leaf("restrict", append([]any{param, "matches"}, list...), nil)
	})
}

func lowerProviders(w *writer, v any, where string) error {
	return each(v, where, func(m *omap, at string) error {
		name, err := requireKey(m, "name", at)
		if err != nil {
			return err
		}
		argv, err := requireKey(m, "exec", at)
		if err != nil {
			return err
		}
		if err := checkKeys(m, at, map[string]fieldFn{"name": skip, "exec": skip}); err != nil {
			return err
		}
		args, err := asStrings(argv, at+".exec")
		if err != nil {
			return err
		}
		closeBlock, err := w.node("provider", []any{name}, nil, true)
		if err != nil {
			return err
		}
		if err := w.leaf("exec", args, nil); err != nil {
			return err
		}
		closeBlock()
		return nil
	})
}

// grants --------------------------------------------------------------------

var grantFields map[string]fieldFn

func init() {
	grantFields = map[string]fieldFn{
		"verb":         skip,
		"resource":     skip,
		"qualifiers":   skip,
		"props":        skip,
		"op":           lowerOp,
		"path":         simple("path"),
		"method":       simple("method"),
		"describe":     simple("describe"),
		"message":      simple("message"),
		"fail_when":    simple("fail-when"),
		"raw_response": lowerFlag("raw-response"),
		"query":        lowerInputs("query"),
		"body":         lowerInputs("body"),
		"fixed_body":   props("body"),
		"set":          props("set"),
		"kdl":          rawKDL,
	}
}

// lowerGrants lowers a list of grants under one modal. An `override` reads
// `override can <verb> <resource>`, so it carries a leading `can`.
func lowerGrants(modal string) fieldFn {
	return func(w *writer, v any, where string) error {
		return each(v, where, func(m *omap, at string) error {
			return lowerGrant(w, modal, m, at)
		})
	}
}

func lowerGrant(w *writer, modal string, m *omap, at string) error {
	args, err := grantArgs(m, modal, at)
	if err != nil {
		return err
	}
	var pr []kv
	if pv, ok := m.get("props"); ok {
		pm, err := asMap(pv, at+".props")
		if err != nil {
			return err
		}
		for _, k := range pm.keys {
			pr = append(pr, kv{k, pm.vals[k]})
		}
	}
	closeBlock, err := w.node(modal, args, pr, hasGrantBody(m))
	if err != nil {
		return err
	}
	if err := lowerFields(w, m, at, grantFields); err != nil {
		return err
	}
	closeBlock()
	return nil
}

func grantArgs(m *omap, modal, at string) ([]any, error) {
	verb, err := requireKey(m, "verb", at)
	if err != nil {
		return nil, err
	}
	resource, err := requireKey(m, "resource", at)
	if err != nil {
		return nil, err
	}
	args := []any{verb, resource}
	if q, ok := m.get("qualifiers"); ok {
		more, err := asStrings(q, at+".qualifiers")
		if err != nil {
			return nil, err
		}
		args = append(args, more...)
	}
	if modal == "override" {
		args = append([]any{"can"}, args...)
	}
	return args, nil
}

// hasGrantBody reports whether a grant has any key that lowers to a child node,
// as opposed to the head keys that ride on the grant node itself.
func hasGrantBody(m *omap) bool {
	for _, k := range m.keys {
		switch k {
		case "verb", "resource", "qualifiers", "props":
		default:
			return true
		}
	}
	return false
}

// lowerFlag writes a bare node when the value is true and nothing when false.
func lowerFlag(name string) fieldFn {
	return func(w *writer, v any, where string) error {
		on, err := asBool(v, where)
		if err != nil || !on {
			return err
		}
		return w.leaf(name, nil, nil)
	}
}

var opFields = map[string]fieldFn{"method": skip, "path": skip}

// lowerOp writes `op "<operationId>"`, or `op method= path=` from a mapping.
func lowerOp(w *writer, v any, where string) error {
	m, ok := v.(*omap)
	if !ok {
		return simple("op")(w, v, where)
	}
	if err := checkKeys(m, where, opFields); err != nil {
		return err
	}
	return props("op")(w, v, where)
}

// lowerInputs writes a `query` or `body` node: names for the flat form, or a
// list of typed entries. See docs/guardfile-formats.md.
func lowerInputs(name string) fieldFn {
	return func(w *writer, v any, where string) error {
		list, typed := v.([]any)
		if !typed || len(list) == 0 {
			return names(name)(w, v, where)
		}
		if _, isEntry := list[0].(*omap); !isEntry {
			return names(name)(w, v, where)
		}
		closeBlock, err := w.node(name, nil, nil, true)
		if err != nil {
			return err
		}
		if err := lowerTypedEntries(w, list, where); err != nil {
			return err
		}
		closeBlock()
		return nil
	}
}

var typedKinds = []string{"field", "array", "object", "map"}

func lowerTypedEntries(w *writer, entries []any, where string) error {
	return each(entries, where, func(m *omap, at string) error {
		return lowerTypedEntry(w, m, at)
	})
}

// typedKind names the one kind an entry declares.
func typedKind(m *omap, where string) (string, error) {
	kind := ""
	for _, k := range typedKinds {
		if _, ok := m.get(k); !ok {
			continue
		}
		if kind != "" {
			return "", fmt.Errorf("%s: names both %q and %q, want one kind", where, kind, k)
		}
		kind = k
	}
	if kind == "" {
		return "", fmt.Errorf("%s: names no kind, want one of %s", where, strings.Join(typedKinds, ", "))
	}
	return kind, nil
}

// lowerTypedEntry writes `field "name" type="string" ...`. Every key besides the
// kind and `entries` is a property, so a new bound needs no change here.
func lowerTypedEntry(w *writer, m *omap, where string) error {
	kind, err := typedKind(m, where)
	if err != nil {
		return err
	}
	name, err := asString(m.vals[kind], where+"."+kind)
	if err != nil {
		return err
	}
	var pr []kv
	var children []any
	for _, k := range m.keys {
		switch k {
		case kind:
		case "entries":
			if children, err = asList(m.vals[k], where+".entries"); err != nil {
				return err
			}
		default:
			pr = append(pr, kv{kebab(k), m.vals[k]})
		}
	}
	closeBlock, err := w.node(kind, []any{name}, pr, children != nil)
	if err != nil {
		return err
	}
	if children != nil {
		if err := lowerTypedEntries(w, children, where+".entries"); err != nil {
			return err
		}
	}
	closeBlock()
	return nil
}
