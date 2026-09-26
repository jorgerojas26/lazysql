package components

import "strings"

// queryReturnsRows classifies editor queries, not their safety. Read-only
// validation must still run independently before either execution path.
func queryReturnsRows(query string) bool {
	tokens := sqlRoutingTokens(query)
	if len(tokens) == 0 {
		return false
	}

	switch tokens[0] {
	case "select", "with", "explain", "show", "describe", "desc", "values", "pragma":
		return true
	case "insert", "update", "delete":
		for _, token := range tokens[1:] {
			if token == "returning" {
				return true
			}
		}
	}
	return false
}

// sqlRoutingTokens returns unquoted words for result-set routing. Comments and
// quoted values/identifiers are skipped rather than stripped with regexes: a
// comment marker inside a string is data, and quoted RETURNING is not a clause.
// This is deliberately not a SQL parser or a read-only security check.
func sqlRoutingTokens(query string) []string {
	var tokens []string
	for i := 0; i < len(query); {
		switch {
		case strings.HasPrefix(query[i:], "--"):
			for i < len(query) && query[i] != '\n' && query[i] != '\r' {
				i++
			}
		case strings.HasPrefix(query[i:], "/*"):
			i += 2
			depth := 1
			for i < len(query) && depth > 0 {
				switch {
				case strings.HasPrefix(query[i:], "/*"):
					depth++
					i += 2
				case strings.HasPrefix(query[i:], "*/"):
					depth--
					i += 2
				default:
					i++
				}
			}
		case query[i] == '\'' || query[i] == '"' || query[i] == '`' || query[i] == '[':
			quote := query[i]
			// PostgreSQL E'...' strings also support backslash escapes.
			escaped := quote == '\'' && i > 0 && (query[i-1] == 'e' || query[i-1] == 'E') &&
				(i == 1 || !sqlRoutingWordByte(query[i-2]))
			if quote == '[' {
				quote = ']'
			}
			i++
			for i < len(query) {
				if escaped && query[i] == '\\' && i+1 < len(query) {
					i += 2
				} else if query[i] == quote {
					i++
					if i < len(query) && query[i] == quote {
						i++ // SQL doubles the delimiter to escape it.
						continue
					}
					break
				} else {
					i++
				}
			}
		case query[i] == '$':
			// PostgreSQL dollar-quoted strings: $$...$$ or $tag$...$tag$.
			end := i + 1
			for end < len(query) && query[end] != '$' && sqlRoutingWordByte(query[end]) {
				end++
			}
			if end < len(query) && query[end] == '$' &&
				(end == i+1 || query[i+1] < '0' || query[i+1] > '9') {
				delimiter := query[i : end+1]
				i = end + 1
				if closeAt := strings.Index(query[i:], delimiter); closeAt >= 0 {
					i += closeAt + len(delimiter)
				} else {
					i = len(query)
				}
			} else {
				i++
			}
		case sqlRoutingWordByte(query[i]):
			start := i
			for i < len(query) && sqlRoutingWordByte(query[i]) {
				i++
			}
			tokens = append(tokens, strings.ToLower(query[start:i]))
		default:
			i++
		}
	}
	return tokens
}

func sqlRoutingWordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' ||
		c >= '0' && c <= '9' || c == '_' || c == '$' || c >= 0x80
}
