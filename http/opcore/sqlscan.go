package opcore

import "strings"

// sqlScanner walks statement text outside string literals, quoted identifiers,
// and comments, so a `;` or `?` inside one is never mistaken for syntax.
type sqlScanner struct {
	src string
	pos int
}

// skipNonCode advances past one literal or comment when the cursor is on it,
// reporting whether it moved.
func (s *sqlScanner) skipNonCode() bool {
	rest := s.src[s.pos:]
	switch {
	case strings.HasPrefix(rest, "--"):
		if end := strings.IndexByte(rest, '\n'); end >= 0 {
			s.pos += end + 1
		} else {
			s.pos = len(s.src)
		}
		return true
	case strings.HasPrefix(rest, "/*"):
		if end := strings.Index(rest[2:], "*/"); end >= 0 {
			s.pos += 2 + end + 2
		} else {
			s.pos = len(s.src)
		}
		return true
	case rest[0] == '\'' || rest[0] == '"' || rest[0] == '`':
		s.skipQuoted(rest[0])
		return true
	}
	return false
}

// skipQuoted consumes a quoted run, treating a doubled quote as an escaped one
// the way SQL does rather than as a terminator.
func (s *sqlScanner) skipQuoted(quote byte) {
	s.pos++
	for s.pos < len(s.src) {
		if s.src[s.pos] == quote {
			if s.pos+1 < len(s.src) && s.src[s.pos+1] == quote {
				s.pos += 2
				continue
			}
			s.pos++
			return
		}
		if s.src[s.pos] == '\\' && quote == '\'' {
			s.pos += 2
			continue
		}
		s.pos++
	}
}

// nextSemicolon reports the offset of the next statement-terminating `;`.
func (s *sqlScanner) nextSemicolon() (int, bool) {
	for s.pos < len(s.src) {
		if s.skipNonCode() {
			continue
		}
		if s.src[s.pos] == ';' {
			at := s.pos
			s.pos++
			return at, true
		}
		s.pos++
	}
	return 0, false
}

// placeholders collects the distinct `$N` numbers and counts bare `?` marks.
func (s *sqlScanner) placeholders() (map[int]bool, int) {
	numbered := map[int]bool{}
	question := 0
	for s.pos < len(s.src) {
		if s.skipNonCode() {
			continue
		}
		switch s.src[s.pos] {
		case '$':
			s.pos++
			start := s.pos
			for s.pos < len(s.src) && s.src[s.pos] >= '0' && s.src[s.pos] <= '9' {
				s.pos++
			}
			if n := s.src[start:s.pos]; n != "" {
				numbered[atoiSafe(n)] = true
			}
		case '?':
			question++
			s.pos++
		default:
			s.pos++
		}
	}
	return numbered, question
}

// firstKeyword reports the statement's opening keyword, lowercased, skipping
// comments and any leading parentheses a wrapped SELECT carries.
func (s *sqlScanner) firstKeyword() string {
	for s.pos < len(s.src) {
		if s.skipNonCode() {
			continue
		}
		c := s.src[s.pos]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '(' {
			s.pos++
			continue
		}
		start := s.pos
		for s.pos < len(s.src) && isSQLWordChar(s.src[s.pos]) {
			s.pos++
		}
		return strings.ToLower(s.src[start:s.pos])
	}
	return ""
}

// sqlWriteKeywords are the mutations a CTE can hide behind a leading `WITH`.
var sqlWriteKeywords = map[string]bool{
	"insert": true, "update": true, "delete": true, "merge": true,
	"truncate": true, "drop": true, "alter": true, "create": true, "grant": true,
}

// mentionsWriteKeyword reports whether any code-position word is a mutation,
// which is how `WITH x AS (...) DELETE ...` is refused for a reading verb.
func (s *sqlScanner) mentionsWriteKeyword() bool {
	for s.pos < len(s.src) {
		if s.skipNonCode() {
			continue
		}
		if !isSQLWordChar(s.src[s.pos]) {
			s.pos++
			continue
		}
		start := s.pos
		for s.pos < len(s.src) && isSQLWordChar(s.src[s.pos]) {
			s.pos++
		}
		if sqlWriteKeywords[strings.ToLower(s.src[start:s.pos])] {
			return true
		}
	}
	return false
}

func isSQLWordChar(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// atoiSafe parses a digit run the scanner already validated, saturating rather
// than erroring since a wild number becomes a placeholder-count mismatch.
func atoiSafe(digits string) int {
	n := 0
	for _, c := range digits {
		n = n*10 + int(c-'0')
		if n > maxSQLRows*1000 {
			return n
		}
	}
	return n
}
