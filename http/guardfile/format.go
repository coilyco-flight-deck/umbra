// format: read a guardfile authored in YAML or TOML by lowering it to KDL text.
// The KDL loader stays the only owner of grammar semantics, so a YAML or TOML
// guardfile means exactly what its KDL twin means, and every load-time check
// (unknown nodes, missing fields, fail-closed rules) still runs. The schema is
// in format_lower.go and the text writer in format_emit.go. See
// docs/guardfile-formats.md.

package guardfile

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// ErrNotGuardfile marks a YAML or TOML file that is not a guardfile, which
// project discovery skips as unrelated (KDL without `wrap` is skipped alike).
var ErrNotGuardfile = errors.New("guardfile: not a guardfile")

// Lower returns KDL source for the guardfile at path. KDL and any unrecognised
// extension come back verbatim, and .yaml, .yml, and .toml are decoded and lowered.
func Lower(path string, src []byte) ([]byte, error) {
	tree, ok, err := decodeByExtension(path, src)
	if err != nil {
		return nil, fmt.Errorf("guardfile: %s: %w", filepath.Base(path), err)
	}
	if !ok {
		return src, nil
	}
	out, err := lowerDocument(tree)
	if err != nil {
		return nil, fmt.Errorf("guardfile: %s: %w", filepath.Base(path), err)
	}
	return out, nil
}

// LowerForDiscovery is Lower for a project walk: a member is a top-level `wrap`
// mapping naming its `command`, other files yield ErrNotGuardfile, malformed ones fail.
func LowerForDiscovery(path string, src []byte) ([]byte, error) {
	tree, ok, err := decodeByExtension(path, src)
	if !ok {
		return src, nil
	}
	if err != nil {
		if formatIntent(src) {
			return nil, fmt.Errorf("guardfile: %s: %w", filepath.Base(path), err)
		}
		return nil, ErrNotGuardfile
	}
	if !namesWrapCommand(tree) {
		return nil, ErrNotGuardfile
	}
	return Lower(path, src)
}

func namesWrapCommand(tree any) bool {
	root, ok := tree.(*omap)
	if !ok {
		return false
	}
	wrap, ok := root.get("wrap")
	if !ok {
		return false
	}
	body, ok := wrap.(*omap)
	if !ok {
		return false
	}
	_, named := body.get("command")
	return named
}

// IsFormatExtension reports whether path names a YAML or TOML guardfile.
func IsFormatExtension(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml", ".toml":
		return true
	}
	return false
}

// formatIntent reports whether malformed YAML or TOML clearly opens a `wrap`, so
// it fails and is not skipped as unrelated.
func formatIntent(src []byte) bool {
	for _, line := range strings.Split(string(src), "\n") {
		line = strings.TrimSpace(line)
		for _, opener := range []string{"wrap:", "[wrap", "wrap =", "wrap="} {
			if strings.HasPrefix(line, opener) {
				return true
			}
		}
	}
	return false
}

func decodeByExtension(path string, src []byte) (tree any, recognised bool, err error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		tree, err = decodeYAML(src)
		return tree, true, err
	case ".toml":
		tree, err = decodeTOML(src)
		return tree, true, err
	}
	return nil, false, nil
}

// omap is an insertion-ordered mapping. YAML and TOML mappings are unordered in
// most decoders, but a guardfile's author order is worth keeping.
type omap struct {
	keys []string
	vals map[string]any
}

// null stands for an explicit YAML null, which lowers to KDL `#null`.
type null struct{}

func newOmap() *omap { return &omap{vals: map[string]any{}} }

func (m *omap) set(key string, val any) error {
	if _, dup := m.vals[key]; dup {
		return fmt.Errorf("duplicate key %q", key)
	}
	m.keys = append(m.keys, key)
	m.vals[key] = val
	return nil
}

func (m *omap) get(key string) (any, bool) {
	v, ok := m.vals[key]
	return v, ok
}

func decodeYAML(src []byte) (any, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(src, &root); err != nil {
		return nil, fmt.Errorf("parse YAML: %w", err)
	}
	if root.Kind == 0 {
		return newOmap(), nil
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) != 1 {
		return nil, errors.New("parse YAML: want exactly one document")
	}
	return fromYAML(root.Content[0])
}

func fromYAML(n *yaml.Node) (any, error) {
	switch n.Kind {
	case yaml.AliasNode:
		return fromYAML(n.Alias)
	case yaml.MappingNode:
		return yamlMapping(n)
	case yaml.SequenceNode:
		list := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			v, err := fromYAML(c)
			if err != nil {
				return nil, err
			}
			list = append(list, v)
		}
		return list, nil
	case yaml.ScalarNode:
		return yamlScalar(n)
	case yaml.DocumentNode:
		return nil, fmt.Errorf("line %d: a document inside a document", n.Line)
	}
	return nil, fmt.Errorf("line %d: unsupported YAML node", n.Line)
}

// yamlMapping refuses merge keys and duplicate keys, because either changes what
// a rule means without showing it in the file.
func yamlMapping(n *yaml.Node) (any, error) {
	m := newOmap()
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i]
		if k.Kind != yaml.ScalarNode {
			return nil, fmt.Errorf("line %d: a mapping key must be a plain string", k.Line)
		}
		if k.Value == "<<" {
			return nil, fmt.Errorf("line %d: YAML merge keys are not supported", k.Line)
		}
		v, err := fromYAML(n.Content[i+1])
		if err != nil {
			return nil, err
		}
		if err := m.set(k.Value, v); err != nil {
			return nil, fmt.Errorf("line %d: %w", k.Line, err)
		}
	}
	return m, nil
}

func yamlScalar(n *yaml.Node) (any, error) {
	var out any
	var err error
	switch n.ShortTag() {
	case "!!null":
		return null{}, nil
	case "!!bool":
		var b bool
		err = n.Decode(&b)
		out = b
	case "!!int":
		var i int64
		err = n.Decode(&i)
		out = i
	case "!!float":
		var f float64
		err = n.Decode(&f)
		out = f
	case "!!str":
		return n.Value, nil
	default:
		return nil, fmt.Errorf("line %d: unsupported YAML value type %s", n.Line, n.ShortTag())
	}
	if err != nil {
		return nil, fmt.Errorf("line %d: %w", n.Line, err)
	}
	return out, nil
}

func decodeTOML(src []byte) (any, error) {
	var raw map[string]any
	md, err := toml.Decode(string(src), &raw)
	if err != nil {
		return nil, fmt.Errorf("parse TOML: %w", err)
	}
	order := map[string]int{}
	for i, k := range md.Keys() {
		if _, seen := order[k.String()]; !seen {
			order[k.String()] = i
		}
	}
	return fromTOML(raw, nil, order)
}

// fromTOML restores document order from the decoder's key list, which an
// array-of-tables entry shares by path, so its keys follow first appearance.
func fromTOML(v any, path []string, order map[string]int) (any, error) {
	switch t := v.(type) {
	case map[string]any:
		return tomlTable(t, path, order)
	case []map[string]any:
		list := make([]any, 0, len(t))
		for _, e := range t {
			list = append(list, e)
		}
		return tomlList(list, path, order)
	case []any:
		return tomlList(t, path, order)
	case string, bool, int64, float64:
		return t, nil
	}
	return nil, fmt.Errorf("unsupported TOML value %T at %s", v, strings.Join(path, "."))
}

func tomlList(items []any, path []string, order map[string]int) (any, error) {
	list := make([]any, 0, len(items))
	for _, e := range items {
		child, err := fromTOML(e, path, order)
		if err != nil {
			return nil, err
		}
		list = append(list, child)
	}
	return list, nil
}

func tomlTable(t map[string]any, path []string, order map[string]int) (any, error) {
	at := func(k string) int {
		if i, ok := order[toml.Key(append(append([]string{}, path...), k)).String()]; ok {
			return i
		}
		return 1 << 30
	}
	keys := make([]string, 0, len(t))
	for k := range t {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if at(keys[i]) != at(keys[j]) {
			return at(keys[i]) < at(keys[j])
		}
		return keys[i] < keys[j]
	})
	m := newOmap()
	for _, k := range keys {
		child, err := fromTOML(t[k], append(append([]string{}, path...), k), order)
		if err != nil {
			return nil, err
		}
		if err := m.set(k, child); err != nil {
			return nil, err
		}
	}
	return m, nil
}
