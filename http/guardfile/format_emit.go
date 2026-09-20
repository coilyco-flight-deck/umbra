// format_emit: the KDL text writer behind format.go's lowering. Every string
// reaches the output through quote, so a value is one argument and never syntax.

package guardfile

import (
	"fmt"
	"strconv"
	"strings"
)

// kv is one property, in author order.
type kv struct {
	key string
	val any
}

// writer builds KDL text with two-space indentation.
type writer struct {
	b     strings.Builder
	depth int
}

func (w *writer) line(s string) {
	w.b.WriteString(strings.Repeat("  ", w.depth))
	w.b.WriteString(s)
	w.b.WriteByte('\n')
}

// node writes `name args... props...`, opening a block when block is true. The
// returned func closes that block, and does nothing for a childless node.
func (w *writer) node(name string, args []any, props []kv, block bool) (func(), error) {
	var sb strings.Builder
	sb.WriteString(ident(name))
	for _, a := range args {
		s, err := scalar(a)
		if err != nil {
			return nil, err
		}
		sb.WriteString(" " + s)
	}
	for _, p := range props {
		s, err := scalar(p.val)
		if err != nil {
			return nil, fmt.Errorf("property %q: %w", p.key, err)
		}
		sb.WriteString(" " + ident(p.key) + "=" + s)
	}
	if !block {
		w.line(sb.String())
		return func() {}, nil
	}
	w.line(sb.String() + " {")
	w.depth++
	return func() { w.depth--; w.line("}") }, nil
}

// leaf writes a childless node.
func (w *writer) leaf(name string, args []any, props []kv) error {
	_, err := w.node(name, args, props, false)
	return err
}

func kebab(s string) string { return strings.ReplaceAll(s, "_", "-") }

// ident renders a node or property name, quoting it unless it is a plain
// identifier that KDL will not read as a keyword or number.
func ident(s string) string {
	if s == "" || keywordLike(s) || !plainIdent(s) {
		return quote(s)
	}
	return s
}

func keywordLike(s string) bool {
	switch s {
	case "true", "false", "null", "inf", "-inf", "nan":
		return true
	}
	return false
}

func plainIdent(s string) bool {
	for i, r := range s {
		letter := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r == '_'
		rest := r >= '0' && r <= '9' || r == '-' || r == '.'
		if !letter && (i == 0 || !rest) {
			return false
		}
	}
	return true
}

func quote(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		default:
			if r < 0x20 || r == 0x7f {
				fmt.Fprintf(&b, `\u{%x}`, r)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

func scalar(v any) (string, error) {
	switch t := v.(type) {
	case string:
		return quote(t), nil
	case bool:
		if t {
			return "#true", nil
		}
		return "#false", nil
	case int64:
		return strconv.FormatInt(t, 10), nil
	case float64:
		s := strconv.FormatFloat(t, 'g', -1, 64)
		if !strings.ContainsAny(s, ".eE") {
			s += ".0"
		}
		return s, nil
	case null:
		return "#null", nil
	}
	return "", fmt.Errorf("want a string, number, or boolean, got %s", describe(v))
}

func describe(v any) string {
	switch v.(type) {
	case *omap:
		return "a mapping"
	case []any:
		return "a list"
	}
	return fmt.Sprintf("%T", v)
}
