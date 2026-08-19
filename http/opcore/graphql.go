package opcore

import (
	"encoding/json"
	"fmt"
	"strings"

	kdl "github.com/calico32/kdl-go"
)

// GraphQL is one grant's operation: an authored document that never reaches
// the input schema, plus caller variables derived from its signature.
type GraphQL struct {
	Document  string  // the operation document, sent verbatim as `query`
	Operation string  // operation name, sent as `operationName`; "" when anonymous
	Variables []Field // derived from the document signature, decorated by `variable`
}

// parseGraphQL reads a `graphql { document ...; variable ... }` grant child.
// The document owns the variable set, so a block cannot disagree with it.
func parseGraphQL(c *kdl.Node) (*GraphQL, error) {
	document, decorations, order, err := readGraphQLBlock(c)
	if err != nil {
		return nil, err
	}
	operation, vars, err := parseGraphQLDocument(document)
	if err != nil {
		return nil, err
	}
	byName := make(map[string]int, len(vars))
	for i, v := range vars {
		byName[v.Name] = i
	}
	// A decoration for a variable the document does not take is the
	// half-specified block this node exists to refuse.
	for _, name := range order {
		idx, ok := byName[name]
		if !ok {
			return nil, fmt.Errorf("`variable` %q is not declared by the document (fail-closed)", name)
		}
		if err := decorateGraphQLVariable(&vars[idx], decorations[name]); err != nil {
			return nil, fmt.Errorf("`variable` %q: %w", name, err)
		}
	}
	// A non-scalar leaves Type empty: an enum and an input object read alike
	// in a document, so the guardfile names it rather than the engine guessing.
	for i := range vars {
		if vars[i].Type == "" {
			return nil, fmt.Errorf(
				"`document` variable $%s has non-scalar type %q, so state it with `variable %q type=\"...\"` (want string | boolean | integer | number | object | array; fail-closed)",
				vars[i].Name, graphQLNamedType(&vars[i]), vars[i].Name)
		}
	}
	return &GraphQL{Document: document, Operation: operation, Variables: vars}, nil
}

// graphQLBlock accumulates one `graphql` node's children while it is read, so
// each child kind validates itself and the loop stays a dispatch.
type graphQLBlock struct {
	document     string
	seenDocument bool
	decorations  map[string]*kdl.Node
	order        []string
}

func (b *graphQLBlock) addDocument(child *kdl.Node) error {
	if b.seenDocument {
		return fmt.Errorf("duplicate `document` (fail-closed)")
	}
	v, err := singleInlineArg(child, "document")
	if err != nil {
		return err
	}
	b.document, b.seenDocument = v, true
	return nil
}

func (b *graphQLBlock) addVariable(child *kdl.Node) error {
	name, err := singleInlineArg(child, "variable")
	if err != nil {
		return err
	}
	if _, dup := b.decorations[name]; dup {
		return fmt.Errorf("duplicate `variable` %q (fail-closed)", name)
	}
	b.decorations[name] = child
	b.order = append(b.order, name)
	return nil
}

// readGraphQLBlock reads the node's children into the document and the
// per-variable decorations, in declared order so errors are deterministic.
func readGraphQLBlock(c *kdl.Node) (string, map[string]*kdl.Node, []string, error) {
	if len(c.Arguments()) != 0 || len(c.Properties()) != 0 {
		return "", nil, nil, fmt.Errorf("`graphql` takes no arguments or properties, only a block (fail-closed)")
	}
	b := &graphQLBlock{decorations: map[string]*kdl.Node{}}
	for _, child := range c.Children().Nodes {
		var err error
		switch child.Name() {
		case "document":
			err = b.addDocument(child)
		case "variable":
			err = b.addVariable(child)
		default:
			err = fmt.Errorf("unknown `graphql` child %q (want document | variable; fail-closed)", child.Name())
		}
		if err != nil {
			return "", nil, nil, err
		}
	}
	if !b.seenDocument {
		return "", nil, nil, fmt.Errorf("`graphql` needs a `document \"...\"` (fail-closed)")
	}
	if strings.TrimSpace(b.document) == "" {
		return "", nil, nil, fmt.Errorf("`graphql` document must be non-empty")
	}
	return b.document, b.decorations, b.order, nil
}

// decorateGraphQLVariable adds only what the document cannot say: help text,
// bounds, and a type for a non-scalar.
func decorateGraphQLVariable(f *Field, c *kdl.Node) error {
	target := f
	// A bound on an array applies to the array itself; a stated type replaces
	// the element type when the document said the element was non-scalar.
	for key, value := range c.Properties() {
		switch key {
		case "type":
			stated := value.String()
			if !validGraphQLStatedType(stated) {
				return fmt.Errorf("type %q is not a schema type (want string | boolean | integer | number | object | array)", stated)
			}
			if err := applyStatedGraphQLType(target, stated); err != nil {
				return err
			}
		case "describe":
			target.Desc = value.String()
		case "minimum", "maximum":
			if err := applyQueryNumericBound(target, key, value.RawValue()); err != nil {
				return err
			}
		case "min-items", "max-items":
			if err := applyQueryArrayBound(target, key, value.RawValue()); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown property %q (want type | describe | minimum | maximum | min-items | max-items; fail-closed)", key)
		}
	}
	return nil
}

// applyStatedGraphQLType fills the unresolved type, reaching the ELEMENT for a
// list: in `[MediaFilter]` the element is the thing needing a name.
func applyStatedGraphQLType(f *Field, stated string) error {
	target := f
	for target.Type == "array" && target.Item != nil {
		target = target.Item
	}
	if target.Type == "array" {
		// A list of a non-scalar: the element type is what was unresolved.
		if target.Items != "" {
			return fmt.Errorf("type is already %q from the document", target.Items)
		}
		if stated == "object" || stated == "array" {
			target.Item = &Field{Name: target.Name, Type: stated, Raw: true}
			return nil
		}
		target.Items = stated
		return nil
	}
	if target.Type != "" {
		return fmt.Errorf("type is already %q from the document", target.Type)
	}
	target.Type = stated
	if stated == "object" || stated == "array" {
		// An input object's inner shape is the upstream's business, so it
		// passes through as an open subtree rather than a guessed schema.
		target.Raw = true
	}
	return nil
}

// validGraphQLStatedType reports whether a `variable type=` value is a schema
// type this engine can carry.
func validGraphQLStatedType(t string) bool {
	switch t {
	case "string", "boolean", "integer", "number", "object", "array":
		return true
	}
	return false
}

// graphQLNamedType reports the document's own spelling for a variable whose
// type this engine could not resolve, so the error can quote it back.
func graphQLNamedType(f *Field) string {
	if f.Type == "array" || f.Items == "" {
		if f.Item != nil {
			return "[" + graphQLNamedType(f.Item) + "]"
		}
		if f.Desc != "" {
			return f.Desc
		}
	}
	return f.Items
}

// parseGraphQLDocument reads the operation name and variable signature. A
// signature reader, not a GraphQL parser: the selection set is never inspected.
func parseGraphQLDocument(document string) (string, []Field, error) {
	s := &graphQLScanner{src: document}
	s.skipIgnored()
	// The shorthand `{ me { id } }` is a valid anonymous query taking no
	// variables, so it is a document with nothing to derive rather than an error.
	if s.peek() == '{' {
		if err := s.checkSingleOperation(); err != nil {
			return "", nil, err
		}
		return "", nil, nil
	}
	keyword := s.readName()
	switch keyword {
	case "query", "mutation", "subscription":
	case "":
		return "", nil, fmt.Errorf("`graphql` document does not start with an operation (want query | mutation | subscription, or a bare `{ ... }`)")
	default:
		return "", nil, fmt.Errorf("`graphql` document starts with %q, which is not an operation (want query | mutation | subscription; fail-closed)", keyword)
	}
	s.skipIgnored()
	operation := ""
	if isGraphQLNameStart(s.peek()) {
		operation = s.readName()
		s.skipIgnored()
	}
	var vars []Field
	if s.peek() == '(' {
		var err error
		if vars, err = s.readVariableDefinitions(); err != nil {
			return "", nil, err
		}
		s.skipIgnored()
	}
	if s.peek() != '{' {
		return "", nil, fmt.Errorf("`graphql` document has no selection set after the operation signature")
	}
	if err := s.checkSingleOperation(); err != nil {
		return "", nil, err
	}
	return operation, vars, nil
}

// graphQLScanner walks a document byte by byte. Deliberately small: it needs to
// find `$name: Type` pairs and balanced braces, not to understand GraphQL.
type graphQLScanner struct {
	src string
	pos int
}

func (s *graphQLScanner) peek() byte {
	if s.pos >= len(s.src) {
		return 0
	}
	return s.src[s.pos]
}

// skipIgnored consumes whitespace, commas (which GraphQL treats as whitespace),
// and `#` comments.
func (s *graphQLScanner) skipIgnored() {
	for s.pos < len(s.src) {
		switch s.src[s.pos] {
		case ' ', '\t', '\n', '\r', ',':
			s.pos++
		case '#':
			for s.pos < len(s.src) && s.src[s.pos] != '\n' {
				s.pos++
			}
		default:
			return
		}
	}
}

func (s *graphQLScanner) readName() string {
	start := s.pos
	if !isGraphQLNameStart(s.peek()) {
		return ""
	}
	for s.pos < len(s.src) && isGraphQLNameChar(s.src[s.pos]) {
		s.pos++
	}
	return s.src[start:s.pos]
}

func isGraphQLNameStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isGraphQLNameChar(c byte) bool {
	return isGraphQLNameStart(c) || (c >= '0' && c <= '9')
}

// readVariableDefinitions reads `($a: String!, $b: Int = 3)` into fields.
func (s *graphQLScanner) readVariableDefinitions() ([]Field, error) {
	s.pos++ // consume '('
	var out []Field
	seen := map[string]bool{}
	for {
		s.skipIgnored()
		if s.peek() == ')' {
			s.pos++
			return out, nil
		}
		if s.peek() != '$' {
			return nil, fmt.Errorf("`graphql` document variable definitions expect `$name: Type`, found %q", s.rest())
		}
		s.pos++
		name := s.readName()
		if name == "" {
			return nil, fmt.Errorf("`graphql` document has a `$` with no variable name")
		}
		if seen[name] {
			return nil, fmt.Errorf("`graphql` document declares $%s twice", name)
		}
		seen[name] = true
		s.skipIgnored()
		if s.peek() != ':' {
			return nil, fmt.Errorf("`graphql` document variable $%s has no `: Type`", name)
		}
		s.pos++
		s.skipIgnored()
		field, required, err := s.readType(name)
		if err != nil {
			return nil, err
		}
		s.skipIgnored()
		// A default makes a non-null variable optional: the server fills it,
		// so requiring it would refuse calls the upstream would have served.
		if s.peek() == '=' {
			s.pos++
			if err := s.skipValue(); err != nil {
				return nil, fmt.Errorf("`graphql` document variable $%s: %w", name, err)
			}
			required = false
		}
		field.Required = required
		out = append(out, field)
	}
}

// readType reads a type reference. It reports whether the OUTERMOST type is
// non-null, the only nullability a caller-facing schema can express.
func (s *graphQLScanner) readType(varName string) (Field, bool, error) {
	f := Field{Name: varName}
	if s.peek() == '[' {
		s.pos++
		s.skipIgnored()
		inner, _, err := s.readType(varName)
		if err != nil {
			return Field{}, false, err
		}
		s.skipIgnored()
		if s.peek() != ']' {
			return Field{}, false, fmt.Errorf("`graphql` document variable $%s has an unclosed list type", varName)
		}
		s.pos++
		f.Type = "array"
		// A scalar element lowers to Items; anything else stays an unresolved
		// Item the guardfile has to name.
		if inner.Type != "" && inner.Item == nil && inner.Type != "array" {
			f.Items = inner.Type
		} else if inner.Type == "array" {
			f.Item = &inner
		}
		return f, s.readNonNull(), nil
	}
	named := s.readName()
	if named == "" {
		return Field{}, false, fmt.Errorf("`graphql` document variable $%s has no type name", varName)
	}
	if scalar, ok := graphQLScalarType(named); ok {
		f.Type = scalar
	} else {
		// Unresolved on purpose: parseGraphQL refuses it unless a `variable`
		// states the type. Desc carries the spelling so the error can quote it.
		f.Desc = named
	}
	return f, s.readNonNull(), nil
}

func (s *graphQLScanner) readNonNull() bool {
	nonNull := false
	for {
		s.skipIgnored()
		if s.peek() != '!' {
			return nonNull
		}
		s.pos++
		nonNull = true
	}
}

// graphQLScalarType maps the built-in scalars. Custom ones are absent because
// `DateTime` is a string to one API and an integer to another.
func graphQLScalarType(named string) (string, bool) {
	switch named {
	case "String", "ID":
		return "string", true
	case "Int":
		return "integer", true
	case "Float":
		return "number", true
	case "Boolean":
		return "boolean", true
	}
	return "", false
}

// skipValue consumes one default value: a string, a list, an object, or a bare
// scalar or enum token.
func (s *graphQLScanner) skipValue() error {
	s.skipIgnored()
	switch s.peek() {
	case '"':
		return s.skipString()
	case '[', '{':
		return s.skipBalanced()
	case 0:
		return fmt.Errorf("default value is missing")
	default:
		start := s.pos
		for s.pos < len(s.src) && !strings.ContainsRune(" \t\r\n,)]}", rune(s.src[s.pos])) {
			s.pos++
		}
		if s.pos == start {
			return fmt.Errorf("default value is missing")
		}
		return nil
	}
}

// skipString consumes a GraphQL string, block strings included, so a `$` or a
// brace inside one is never mistaken for syntax.
func (s *graphQLScanner) skipString() error {
	if strings.HasPrefix(s.src[s.pos:], `"""`) {
		end := strings.Index(s.src[s.pos+3:], `"""`)
		if end < 0 {
			return fmt.Errorf("unterminated block string")
		}
		s.pos += 3 + end + 3
		return nil
	}
	s.pos++ // consume the opening quote
	for s.pos < len(s.src) {
		switch s.src[s.pos] {
		case '\\':
			s.pos += 2
			continue
		case '"':
			s.pos++
			return nil
		case '\n':
			return fmt.Errorf("unterminated string")
		}
		s.pos++
	}
	return fmt.Errorf("unterminated string")
}

// skipBalanced consumes one bracketed run, honouring strings and comments so a
// brace inside either does not unbalance the count.
func (s *graphQLScanner) skipBalanced() error {
	opener := s.peek()
	closer := byte('}')
	if opener == '[' {
		closer = ']'
	}
	depth := 0
	for s.pos < len(s.src) {
		switch s.src[s.pos] {
		case '"':
			if err := s.skipString(); err != nil {
				return err
			}
			continue
		case '#':
			for s.pos < len(s.src) && s.src[s.pos] != '\n' {
				s.pos++
			}
			continue
		case opener:
			depth++
		case closer:
			depth--
			if depth == 0 {
				s.pos++
				return nil
			}
		}
		s.pos++
	}
	return fmt.Errorf("unbalanced %q", string(opener))
}

// checkSingleOperation consumes the current selection set and refuses a second
// operation after it. Fragment definitions are legitimate and pass through.
func (s *graphQLScanner) checkSingleOperation() error {
	if err := s.skipBalanced(); err != nil {
		return fmt.Errorf("`graphql` document has an %w selection set", err)
	}
	for {
		s.skipIgnored()
		if s.peek() == 0 {
			return nil
		}
		switch name := s.readName(); name {
		case "query", "mutation", "subscription":
			return fmt.Errorf("`graphql` document declares more than one operation, so `operationName` would decide which one runs (state one operation per grant; fail-closed)")
		case "fragment":
			s.skipIgnored()
			for s.pos < len(s.src) && s.peek() != '{' {
				s.pos++
			}
			if err := s.skipBalanced(); err != nil {
				return fmt.Errorf("`graphql` document has an %w fragment", err)
			}
		default:
			return fmt.Errorf("`graphql` document has trailing content after the operation: %q", s.rest())
		}
	}
}

// rest quotes a short window of what is left, for an actionable parse error.
func (s *graphQLScanner) rest() string {
	tail := strings.TrimSpace(s.src[min(s.pos, len(s.src)):])
	if len(tail) > 40 {
		return tail[:40] + "..."
	}
	return tail
}

// validateGraphQLGrant refuses a grant whose body has two owners, and one that
// would send a GraphQL document by a method that cannot carry it.
func validateGraphQLGrant(d Descriptor, verb, resource string) error {
	if d.GraphQL == nil {
		return nil
	}
	for _, conflict := range []struct {
		present bool
		node    string
	}{
		{len(d.BodyFlags) > 0, "body"},
		{len(d.BodyMappings) > 0, "body `map`"},
		{len(d.FixedBody) > 0, "set"},
	} {
		if conflict.present {
			return fmt.Errorf("opcore: can %s %s: `graphql` owns the request body, so it cannot be combined with `%s` (fail-closed)", verb, resource, conflict.node)
		}
	}
	// GraphQL over GET puts the document in the query string, which this node
	// does not build. AniList answers one with a 404 rather than a reason.
	if d.Method != "POST" {
		return fmt.Errorf("opcore: can %s %s: `graphql` sends the document as a POST body, but this grant's method is %s (use a post verb, or state `method \"POST\"`; fail-closed)", verb, resource, d.Method)
	}
	return nil
}

// assembleGraphQLBody builds the one body shape a GraphQL upstream accepts.
// Only DECLARED variables are forwarded, so an undeclared input cannot ride.
func assembleGraphQLBody(body map[string]any, gql *GraphQL) ([]byte, error) {
	if err := validateBodyFields(body, gql.Variables, ""); err != nil {
		return nil, err
	}
	out := map[string]any{"query": gql.Document}
	if gql.Operation != "" {
		out["operationName"] = gql.Operation
	}
	if len(gql.Variables) > 0 {
		vars := make(map[string]any, len(gql.Variables))
		for _, v := range gql.Variables {
			if value, ok := body[v.Name]; ok {
				vars[v.Name] = value
			}
		}
		out["variables"] = vars
	}
	return json.Marshal(out)
}
