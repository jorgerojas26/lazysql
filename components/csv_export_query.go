package components

import "strings"

// isReplaySafeQuery accepts only result-producing SQL whose text can be
// conservatively replayed for an Export All Results operation. It is
// intentionally stricter than the read-only connection validator: unknown
// statements, mutation verbs, and multiple statements are refused rather than
// guessed at.
func isReplaySafeQuery(query string) bool {
	tokens, ok := tokenizeReplayQuery(query)
	if !ok || len(tokens) == 0 {
		return false
	}

	end := len(tokens)
	for i, token := range tokens {
		if token.kind != replayTokenSemicolon {
			continue
		}
		if i != len(tokens)-1 {
			return false
		}
		end = i
	}
	if end == 0 {
		return false
	}
	tokens = tokens[:end]

	if tokens[0].kind != replayTokenWord {
		return false
	}
	first := tokens[0].text
	switch first {
	case "SELECT", "WITH", "EXPLAIN", "SHOW", "DESCRIBE", "DESC":
	default:
		return false
	}

	for i, token := range tokens {
		if token.kind == replayTokenWord {
			if replayUnsafeWord[token.text] {
				return false
			}
			if replayFunctionCall(tokens, i) && !replaySafeFunction[token.text] && !replayStructuralWord[token.text] {
				return false
			}
			continue
		}
		if token.kind == replayTokenQuoted && replayFunctionCall(tokens, i) {
			// A quoted function name cannot be classified without resolving it
			// against the connected database, so do not replay it automatically.
			return false
		}
	}

	// A WITH query is only replayable when its final statement is a SELECT.
	// Looking for a top-level SELECT avoids accepting a CTE body alone while
	// still allowing nested SELECTs inside the CTE and the result query.
	if first == "WITH" {
		for _, token := range tokens[1:] {
			if token.kind == replayTokenWord && token.depth == 0 && token.text == "SELECT" {
				return true
			}
		}
		return false
	}

	return true
}

type replayTokenKind uint8

const (
	replayTokenWord replayTokenKind = iota
	replayTokenPunctuation
	replayTokenSemicolon
	replayTokenQuoted
)

type replayToken struct {
	text  string
	kind  replayTokenKind
	depth int
}

// These words either mutate state directly or commonly represent a
// potentially side-effecting statement. The list is deliberately broader than
// drivers.IsQueryMutation because Export All must never guess about unknown
// result-producing SQL.
var replayUnsafeWord = map[string]bool{
	"INSERT":    true,
	"UPDATE":    true,
	"DELETE":    true,
	"MERGE":     true,
	"REPLACE":   true,
	"UPSERT":    true,
	"DROP":      true,
	"ALTER":     true,
	"TRUNCATE":  true,
	"CREATE":    true,
	"GRANT":     true,
	"REVOKE":    true,
	"RENAME":    true,
	"CALL":      true,
	"DO":        true,
	"EXEC":      true,
	"EXECUTE":   true,
	"VACUUM":    true,
	"ANALYZE":   true,
	"COPY":      true,
	"LOAD":      true,
	"ATTACH":    true,
	"DETACH":    true,
	"PRAGMA":    true,
	"SET":       true,
	"USE":       true,
	"BEGIN":     true,
	"START":     true,
	"COMMIT":    true,
	"ROLLBACK":  true,
	"SAVEPOINT": true,
	"RELEASE":   true,
	"LOCK":      true,
	"KILL":      true,
	"FLUSH":     true,
	"RESET":     true,
	"DISCARD":   true,
	"REINDEX":   true,
	"REFRESH":   true,
	"INTO":      true, // SELECT INTO can create or overwrite data.
	// Known side-effecting functions in common drivers.
	"NEXTVAL":                true,
	"SETVAL":                 true,
	"PG_ADVISORY_LOCK":       true,
	"PG_TRY_ADVISORY_LOCK":   true,
	"PG_NOTIFY":              true,
	"DBLINK_EXEC":            true,
	"DBLINK_EXECU":           true,
	"LO_CREATE":              true,
	"LO_UNLINK":              true,
	"PG_ADVISORY_UNLOCK":     true,
	"PG_ADVISORY_UNLOCK_ALL": true,
	"PG_SLEEP":               true,
	"SLEEP":                  true,
	"SET_CONFIG":             true,
	"LOAD_EXTENSION":         true,
	"RANDOM":                 true,
	"RAND":                   true,
	"NEWID":                  true,
	"UUID":                   true,
	"GEN_RANDOM_UUID":        true,
}

// SQL grammar uses parentheses for a few constructs that are not function
// calls. All other function names must be known pure built-ins; an arbitrary
// user-defined function can have side effects even when it returns rows.
var replayStructuralWord = map[string]bool{
	"SELECT": true, "FROM": true, "JOIN": true, "ON": true,
	"IN": true, "EXISTS": true, "NOT": true, "AS": true,
	"VALUES": true, "OVER": true, "FILTER": true,
}

var replaySafeFunction = map[string]bool{
	"ABS": true, "ARRAY_AGG": true, "AVG": true, "BOOL_AND": true,
	"BOOL_OR": true, "CAST": true, "COALESCE": true, "CONCAT": true,
	"CONVERT": true, "COUNT": true, "DATE": true, "DATEADD": true,
	"DATEDIFF": true, "DATE_PART": true, "DATE_TRUNC": true,
	"DATETIME": true, "EXTRACT": true, "FORMAT": true,
	"GREATEST": true, "GROUP_CONCAT": true, "IIF": true,
	"IF": true, "IFNULL": true, "ISNULL": true, "JSON_ARRAY": true,
	"JSON_BUILD_ARRAY": true, "JSON_BUILD_OBJECT": true,
	"JSON_EXTRACT": true, "JSON_OBJECT": true, "JSON_VALUE": true,
	"LEAST": true, "LEFT": true, "LENGTH": true, "LEN": true,
	"LOWER": true, "MAX": true, "MIN": true, "NULLIF": true,
	"PRINTF": true, "REGEXP_REPLACE": true, "ROUND": true,
	"SPLIT_PART": true, "STRING_AGG": true, "STRFTIME": true,
	"SUBSTR": true, "SUM": true, "TO_CHAR": true, "TO_DATE": true,
	"TO_TIMESTAMP": true, "TRIM": true, "TYPEOF": true,
	"UNNEST": true, "UPPER": true,
}

func replayFunctionCall(tokens []replayToken, index int) bool {
	return index+1 < len(tokens) &&
		tokens[index+1].kind == replayTokenPunctuation && tokens[index+1].text == "("
}

func tokenizeReplayQuery(query string) ([]replayToken, bool) {
	var tokens []replayToken
	depth := 0
	for i := 0; i < len(query); {
		c := query[i]
		switch {
		case isReplaySpace(c):
			i++
		case c == '-' && i+1 < len(query) && query[i+1] == '-':
			i += 2
			for i < len(query) && query[i] != '\n' {
				i++
			}
		case c == '#':
			// MySQL-style line comments are harmless but must not expose words
			// that look like mutation verbs to the classifier.
			i++
			for i < len(query) && query[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < len(query) && query[i+1] == '*':
			var ok bool
			i, ok = skipReplayBlockComment(query, i+2)
			if !ok {
				return nil, false
			}
		case c == '\'' || c == '"' || c == '`':
			var ok bool
			i, ok = skipReplayQuoted(query, i, c)
			if !ok {
				return nil, false
			}
			tokens = append(tokens, replayToken{kind: replayTokenQuoted, depth: depth})
		case c == '[':
			var ok bool
			i, ok = skipReplayBracketQuote(query, i+1)
			if !ok {
				return nil, false
			}
			tokens = append(tokens, replayToken{kind: replayTokenQuoted, depth: depth})
		case c == '$':
			if replayDollarQuoteStart(query, i) {
				end, ok := skipReplayDollarQuote(query, i)
				if !ok {
					return nil, false
				}
				i = end
				tokens = append(tokens, replayToken{kind: replayTokenQuoted, depth: depth})
				continue
			}
			// A dollar parameter or operator is punctuation, not a word.
			tokens = append(tokens, replayToken{kind: replayTokenPunctuation, depth: depth})
			i++
		case c == ';':
			tokens = append(tokens, replayToken{kind: replayTokenSemicolon, depth: depth})
			i++
		case c == '(':
			tokens = append(tokens, replayToken{text: "(", kind: replayTokenPunctuation, depth: depth})
			depth++
			i++
		case c == ')':
			if depth == 0 {
				return nil, false
			}
			depth--
			tokens = append(tokens, replayToken{text: ")", kind: replayTokenPunctuation, depth: depth})
			i++
		case isReplayWordStart(c):
			start := i
			i++
			for i < len(query) && isReplayWordPart(query[i]) {
				i++
			}
			tokens = append(tokens, replayToken{
				text:  strings.ToUpper(query[start:i]),
				kind:  replayTokenWord,
				depth: depth,
			})
		default:
			tokens = append(tokens, replayToken{text: string(c), kind: replayTokenPunctuation, depth: depth})
			i++
		}
	}
	return tokens, depth == 0
}

func skipReplayBlockComment(query string, start int) (int, bool) {
	depth := 1
	for i := start; i < len(query)-1; i++ {
		if query[i] == '/' && query[i+1] == '*' {
			depth++
			i++
			continue
		}
		if query[i] == '*' && query[i+1] == '/' {
			depth--
			if depth == 0 {
				return i + 2, true
			}
			i++
		}
	}
	return len(query), false
}

func skipReplayQuoted(query string, start int, quote byte) (int, bool) {
	for i := start + 1; i < len(query); i++ {
		if query[i] == '\\' && quote == '\'' && i+1 < len(query) {
			i++
			continue
		}
		if query[i] != quote {
			continue
		}
		if i+1 < len(query) && query[i+1] == quote {
			i++
			continue
		}
		return i + 1, true
	}
	return len(query), false
}

func skipReplayBracketQuote(query string, start int) (int, bool) {
	for i := start; i < len(query); i++ {
		if query[i] != ']' {
			continue
		}
		if i+1 < len(query) && query[i+1] == ']' {
			i++
			continue
		}
		return i + 1, true
	}
	return len(query), false
}

func replayDollarQuoteStart(query string, start int) bool {
	if start+1 >= len(query) {
		return false
	}
	if query[start+1] == '$' {
		return true
	}
	if !isReplayWordStart(query[start+1]) {
		return false
	}
	for i := start + 2; i < len(query); i++ {
		if query[i] == '$' {
			return true
		}
		if !isReplayWordPart(query[i]) {
			return false
		}
	}
	return false
}

func skipReplayDollarQuote(query string, start int) (int, bool) {
	endTag := strings.IndexByte(query[start+1:], '$')
	if endTag < 0 {
		return start, false
	}
	endTag += start + 1
	tag := query[start : endTag+1]
	if len(tag) > 2 {
		for i := 1; i < len(tag)-1; i++ {
			if i == 1 && !isReplayWordStart(tag[i]) {
				return start, false
			}
			if i > 1 && !isReplayWordPart(tag[i]) {
				return start, false
			}
		}
	}
	closeAt := strings.Index(query[endTag+1:], tag)
	if closeAt < 0 {
		return len(query), false
	}
	return endTag + 1 + closeAt + len(tag), true
}

func isReplaySpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '\f'
}

func isReplayWordStart(c byte) bool {
	return c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' || c == '_'
}

func isReplayWordPart(c byte) bool {
	return isReplayWordStart(c) || c >= '0' && c <= '9' || c == '$'
}
