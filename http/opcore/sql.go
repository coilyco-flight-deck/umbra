package opcore

import (
	"fmt"
	"strconv"
	"strings"

	kdl "github.com/calico32/kdl-go"
)

// defaultSQLRows bounds a result set when a guardfile states none, and
// maxSQLRows ceilings what it may raise that to.
const (
	defaultSQLRows = 200
	maxSQLRows     = 10000
)

// SQL is one grant's declared statement: authored text that never reaches the
// input schema, plus the parameters a caller binds into its placeholders.
type SQL struct {
	Statement   string  // sent verbatim; parameters bind, nothing interpolates
	Params      []Field // ordered, binding to $1..$N or ?..? in that order
	MaxRows     int     // result bound for a reading statement
	Reads       bool    // true when the statement returns rows rather than a count
	Placeholder byte    // '$' or '?', the style the statement uses
}

// sqlBlock accumulates one `sql` node's children so the read loop stays a
// dispatch and each child kind validates itself.
type sqlBlock struct {
	statement string
	seenStmt  bool
	rows      int
	seenRows  bool
	params    []Field
	names     map[string]bool
}

func (b *sqlBlock) addStatement(c *kdl.Node) error {
	if b.seenStmt {
		return fmt.Errorf("duplicate `statement` (fail-closed)")
	}
	v, err := singleInlineArg(c, "statement")
	if err != nil {
		return err
	}
	b.statement, b.seenStmt = v, true
	return nil
}

func (b *sqlBlock) addMaxRows(c *kdl.Node) error {
	if b.seenRows {
		return fmt.Errorf("duplicate `max-rows` (fail-closed)")
	}
	v, err := singleInlineArg(c, "max-rows")
	if err != nil {
		return err
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("`max-rows` %q must be a whole number", v)
	}
	if n < 1 || n > maxSQLRows {
		return fmt.Errorf("`max-rows` must be between 1 and %d, got %d", maxSQLRows, n)
	}
	b.rows, b.seenRows = n, true
	return nil
}

// addParam reads one `param "name" type="..."` declaration. Order is the
// binding order, so the guardfile's sequence is the placeholder sequence.
func (b *sqlBlock) addParam(c *kdl.Node) error {
	name, err := singleInlineArg(c, "param")
	if err != nil {
		return err
	}
	if b.names[name] {
		return fmt.Errorf("duplicate `param` %q (fail-closed)", name)
	}
	f := Field{Name: name}
	if err := applySQLParamProperties(&f, c); err != nil {
		return fmt.Errorf("`param` %q: %w", name, err)
	}
	if f.Type == "" {
		return fmt.Errorf("`param` %q needs a `type=` (want string | boolean | integer | number; fail-closed)", name)
	}
	b.names[name] = true
	b.params = append(b.params, f)
	return nil
}

// applySQLParamProperties reads one param's properties. Only scalars bind: a
// driver takes a scalar argument, and an object would have to be serialized.
func applySQLParamProperties(f *Field, c *kdl.Node) error {
	for key, value := range c.Properties() {
		switch key {
		case "type":
			t := value.String()
			if !validSQLParamType(t) {
				return fmt.Errorf("type %q does not bind as a parameter (want string | boolean | integer | number)", t)
			}
			f.Type = t
		case "required":
			b, ok := value.RawValue().(bool)
			if !ok {
				return fmt.Errorf("`required` must be #true or #false")
			}
			f.Required = b
		case "describe":
			f.Desc = value.String()
		case "minimum", "maximum":
			if err := applyQueryNumericBound(f, key, value.RawValue()); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown property %q (want type | required | describe | minimum | maximum; fail-closed)", key)
		}
	}
	return nil
}

func validSQLParamType(t string) bool {
	switch t {
	case "string", "boolean", "integer", "number":
		return true
	}
	return false
}

// parseSQL reads a `sql { statement ...; param ...; max-rows ... }` grant child
// and checks the statement against the parameters the guardfile declared.
func parseSQL(c *kdl.Node) (*SQL, error) {
	if len(c.Arguments()) != 0 || len(c.Properties()) != 0 {
		return nil, fmt.Errorf("`sql` takes no arguments or properties, only a block (fail-closed)")
	}
	b := &sqlBlock{rows: defaultSQLRows, names: map[string]bool{}}
	for _, child := range c.Children().Nodes {
		var err error
		switch child.Name() {
		case "statement":
			err = b.addStatement(child)
		case "param":
			err = b.addParam(child)
		case "max-rows":
			err = b.addMaxRows(child)
		default:
			err = fmt.Errorf("unknown `sql` child %q (want statement | param | max-rows; fail-closed)", child.Name())
		}
		if err != nil {
			return nil, err
		}
	}
	if !b.seenStmt {
		return nil, fmt.Errorf("`sql` needs a `statement \"...\"` (fail-closed)")
	}
	return buildSQL(b)
}

// buildSQL turns the read block into a checked SQL, which is where the
// statement and the declared parameters have to agree.
func buildSQL(b *sqlBlock) (*SQL, error) {
	statement := strings.TrimSpace(b.statement)
	if statement == "" {
		return nil, fmt.Errorf("`sql` statement must be non-empty")
	}
	if err := checkSingleStatement(statement); err != nil {
		return nil, err
	}
	style, count, err := sqlPlaceholders(statement)
	if err != nil {
		return nil, err
	}
	if count != len(b.params) {
		return nil, fmt.Errorf("`sql` statement has %d placeholder(s) but %d `param` declaration(s) (fail-closed)", count, len(b.params))
	}
	reads, err := sqlStatementReads(statement)
	if err != nil {
		return nil, err
	}
	return &SQL{
		Statement:   statement,
		Params:      b.params,
		MaxRows:     b.rows,
		Reads:       reads,
		Placeholder: style,
	}, nil
}

// checkSingleStatement refuses a second statement, so a sloppy authored string
// can never become a stacked query. One trailing `;` is fine.
func checkSingleStatement(statement string) error {
	scan := &sqlScanner{src: statement}
	i, ok := scan.nextSemicolon()
	if !ok {
		return nil
	}
	if strings.TrimSpace(statement[i+1:]) != "" {
		return fmt.Errorf("`sql` statement carries more than one statement (one per grant; fail-closed)")
	}
	return nil
}

// sqlPlaceholders reports the placeholder style and count. `$N` wins when
// present, so a Postgres JSON `?` operator is not miscounted as a parameter.
func sqlPlaceholders(statement string) (byte, int, error) {
	scan := &sqlScanner{src: statement}
	numbered, question := scan.placeholders()
	if len(numbered) > 0 {
		highest := 0
		for n := range numbered {
			if n > highest {
				highest = n
			}
		}
		if highest != len(numbered) {
			return 0, 0, fmt.Errorf("`sql` statement numbers placeholders up to $%d but uses %d distinct one(s): they must be $1..$N (fail-closed)", highest, len(numbered))
		}
		return '$', highest, nil
	}
	return '?', question, nil
}

// sqlReadKeywords open a statement that returns rows rather than changing them.
var sqlReadKeywords = map[string]bool{
	"select": true, "with": true, "show": true, "explain": true,
	"values": true, "table": true, "pragma": true,
}

// sqlStatementReads reports whether the statement returns rows, from its
// leading keyword. `WITH ... DELETE` is a write and is caught below.
func sqlStatementReads(statement string) (bool, error) {
	scan := &sqlScanner{src: statement}
	first := scan.firstKeyword()
	if first == "" {
		return false, fmt.Errorf("`sql` statement does not start with a keyword")
	}
	if !sqlReadKeywords[first] {
		return false, nil
	}
	// A CTE can carry a mutation, and reporting that as a read would let a read
	// verb serve a write. Only a wholly read `WITH` counts as one.
	if first == "with" && scan.mentionsWriteKeyword() {
		return false, nil
	}
	return true, nil
}

// validateSQLGrant refuses a grant whose SQL conflicts with an HTTP construct,
// and a reading verb whose statement writes.
func validateSQLGrant(d Descriptor, verb, resource string) error {
	if d.SQL == nil {
		return nil
	}
	for _, conflict := range []struct {
		present bool
		node    string
	}{
		{d.Path != "", "path"},
		{len(d.QueryFlags) > 0, "query"},
		{len(d.BodyFlags) > 0, "body"},
		{len(d.BodyMappings) > 0, "body `map`"},
		{len(d.FixedBody) > 0, "set"},
		{d.GraphQL != nil, "graphql"},
		{d.RawResponse, "raw-response"},
	} {
		if conflict.present {
			return fmt.Errorf("opcore: can %s %s: `sql` reaches a database rather than a URL, so it cannot be combined with `%s` (fail-closed)", verb, resource, conflict.node)
		}
	}
	// The served verb must not lie about what it does: a caller reading `list`
	// has no reason to expect a mutation behind it.
	if method, _ := MethodForVerb(verb); method == "GET" && !d.SQL.Reads {
		return fmt.Errorf("opcore: can %s %s: %q is a reading verb but the statement mutates (use a writing verb; fail-closed)", verb, resource, verb)
	}
	return nil
}
