package components

import (
	"strings"
	"unicode"

	"github.com/gdamore/tcell/v2"
)

// SQLTokenType classifies a span of SQL text for syntax highlighting.
type SQLTokenType int

const (
	TokenWhitespace SQLTokenType = iota
	TokenKeyword
	TokenString
	TokenNumber
	TokenComment
	TokenFunction
	TokenOperator
	TokenIdentifier
	TokenPunctuation
	TokenParameter // $1, ?
	TokenTypeDef   // INT, VARCHAR, TEXT, etc.
	TokenBoolean   // TRUE, FALSE, NULL
)

// SQLToken represents a single token in SQL source.
type SQLToken struct {
	Type  SQLTokenType
	Start int // byte offset in the input (inclusive)
	End   int // byte offset in the input (exclusive)
}

// sqllLexer tokenizes a SQL string.
type sqllLexer struct {
	input []rune
	pos   int
}

// tokenizeSQL tokenizes a SQL string and returns color styles for each byte.
// It returns a slice parallel to the input bytes mapping each to a tcell.Style.
func tokenizeSQL(input string, defaultFg, defaultBg tcell.Color) []tcell.Style {
	if input == "" {
		return nil
	}

	tokens := tokenize(input)
	runes := []rune(input)
	styles := make([]tcell.Style, len(runes))

	// Default style
	defStyle := tcell.StyleDefault.Foreground(defaultFg).Background(defaultBg)

	// Pre-fill with defaults
	for i := range styles {
		styles[i] = defStyle
	}

	for _, tok := range tokens {
		color := colorForToken(tok.Type)
		for i := tok.Start; i < tok.End && i < len(styles); i++ {
			fg := color
			if fg == tcell.ColorDefault {
				fg = defaultFg
			}
			styles[i] = tcell.StyleDefault.Foreground(fg).Background(defaultBg)
		}
	}

	return styles
}

// tokenize splits SQL input into tokens.
func tokenize(input string) []SQLToken {
	l := &sqllLexer{
		input: []rune(input),
		pos:   0,
	}
	var tokens []SQLToken

	for l.pos < len(l.input) {
		ch := l.input[l.pos]

		switch {
		case ch == '-' && l.peek() == '-':
			tokens = append(tokens, l.readComment())
		case ch == '/' && l.peek() == '*':
			tokens = append(tokens, l.readBlockComment())
		case ch == '\'':
			tokens = append(tokens, l.readString())
		case ch == '"':
			tokens = append(tokens, l.readQuotedIdentifier())
		case ch == '`':
			tokens = append(tokens, l.readBacktickIdentifier())
		case ch == '$' && l.peekRune(1) != '(' && l.peekRune(1) != '\'':
			// Positional parameter like $1
			tokens = append(tokens, l.readParameter())
		case ch == '?':
			tokens = append(tokens, SQLToken{Type: TokenParameter, Start: l.pos, End: l.pos + 1})
			l.pos++
		case ch == ':' && l.pos+1 < len(l.input) && !unicode.IsSpace(l.input[l.pos+1]) && l.input[l.pos+1] != ':' && l.input[l.pos+1] != '=':
			// Named parameter :name
			tokens = append(tokens, l.readNamedParameter())
		case isDigit(ch) || (ch == '.' && l.pos+1 < len(l.input) && isDigit(l.input[l.pos+1])):
			tokens = append(tokens, l.readNumber())
		case isLetter(ch) || ch == '_':
			tokens = append(tokens, l.readIdentOrKeyword())
		case isOperator(ch):
			tokens = append(tokens, l.readOperator())
		case isPunctuation(ch):
			tokens = append(tokens, SQLToken{Type: TokenPunctuation, Start: l.pos, End: l.pos + 1})
			l.pos++
		default:
			// Whitespace or unknown
			start := l.pos
			for l.pos < len(l.input) && (unicode.IsSpace(l.input[l.pos]) || l.input[l.pos] == 0) {
				l.pos++
			}
			if l.pos > start {
				tokens = append(tokens, SQLToken{Type: TokenWhitespace, Start: start, End: l.pos})
			} else {
				l.pos++
			}
		}
	}

	return tokens
}

// --- Lexer helpers ---

func (l *sqllLexer) peek() rune {
	if l.pos+1 < len(l.input) {
		return l.input[l.pos+1]
	}
	return 0
}

func (l *sqllLexer) peekRune(n int) rune {
	if l.pos+n < len(l.input) {
		return l.input[l.pos+n]
	}
	return 0
}

func (l *sqllLexer) readComment() SQLToken {
	start := l.pos
	for l.pos < len(l.input) && l.input[l.pos] != '\n' {
		l.pos++
	}
	return SQLToken{Type: TokenComment, Start: start, End: l.pos}
}

func (l *sqllLexer) readBlockComment() SQLToken {
	start := l.pos
	l.pos += 2 // skip /*
	for l.pos < len(l.input) {
		if l.input[l.pos] == '*' && l.peek() == '/' {
			l.pos += 2
			break
		}
		l.pos++
	}
	return SQLToken{Type: TokenComment, Start: start, End: l.pos}
}

func (l *sqllLexer) readString() SQLToken {
	start := l.pos
	quote := l.input[l.pos]
	l.pos++ // skip opening quote
	for l.pos < len(l.input) {
		ch := l.input[l.pos]
		if ch == quote {
			l.pos++ // skip closing quote
			// Handle escaped quotes: ''
			if l.pos < len(l.input) && l.input[l.pos] == quote {
				l.pos++
				continue
			}
			break
		}
		l.pos++
	}
	return SQLToken{Type: TokenString, Start: start, End: l.pos}
}

func (l *sqllLexer) readQuotedIdentifier() SQLToken {
	start := l.pos
	l.pos++ // skip opening "
	for l.pos < len(l.input) {
		if l.input[l.pos] == '"' {
			l.pos++ // skip closing "
			// Handle escaped quotes: ""
			if l.pos < len(l.input) && l.input[l.pos] == '"' {
				l.pos++
				continue
			}
			break
		}
		l.pos++
	}
	return SQLToken{Type: TokenIdentifier, Start: start, End: l.pos}
}

func (l *sqllLexer) readBacktickIdentifier() SQLToken {
	start := l.pos
	l.pos++ // skip opening `
	for l.pos < len(l.input) {
		if l.input[l.pos] == '`' {
			l.pos++ // skip closing `
			break
		}
		l.pos++
	}
	return SQLToken{Type: TokenIdentifier, Start: start, End: l.pos}
}

func (l *sqllLexer) readParameter() SQLToken {
	start := l.pos
	l.pos++ // skip $
	for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
		l.pos++
	}
	return SQLToken{Type: TokenParameter, Start: start, End: l.pos}
}

func (l *sqllLexer) readNamedParameter() SQLToken {
	start := l.pos
	l.pos++ // skip :
	for l.pos < len(l.input) && (isLetter(l.input[l.pos]) || isDigit(l.input[l.pos]) || l.input[l.pos] == '_') {
		l.pos++
	}
	return SQLToken{Type: TokenParameter, Start: start, End: l.pos}
}

func (l *sqllLexer) readNumber() SQLToken {
	start := l.pos
	// Optional leading dot
	if l.input[l.pos] == '.' {
		l.pos++
	}
	for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
		l.pos++
	}
	// Optional decimal part
	if l.pos < len(l.input) && l.input[l.pos] == '.' {
		l.pos++
		for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
			l.pos++
		}
	}
	// Optional exponent
	if l.pos < len(l.input) && (l.input[l.pos] == 'e' || l.input[l.pos] == 'E') {
		l.pos++
		if l.pos < len(l.input) && (l.input[l.pos] == '+' || l.input[l.pos] == '-') {
			l.pos++
		}
		for l.pos < len(l.input) && isDigit(l.input[l.pos]) {
			l.pos++
		}
	}
	return SQLToken{Type: TokenNumber, Start: start, End: l.pos}
}

func (l *sqllLexer) readIdentOrKeyword() SQLToken {
	start := l.pos
	for l.pos < len(l.input) && (isLetter(l.input[l.pos]) || isDigit(l.input[l.pos]) || l.input[l.pos] == '_') {
		l.pos++
	}
	word := string(l.input[start:l.pos])
	upper := strings.ToUpper(word)

	if isKeyword(upper) {
		return SQLToken{Type: TokenKeyword, Start: start, End: l.pos}
	}
	if isType(upper) {
		return SQLToken{Type: TokenTypeDef, Start: start, End: l.pos}
	}
	if isBoolean(upper) {
		return SQLToken{Type: TokenBoolean, Start: start, End: l.pos}
	}
	// Check if followed by ( -- could be a function call
	if l.pos < len(l.input) && l.input[l.pos] == '(' {
		return SQLToken{Type: TokenFunction, Start: start, End: l.pos}
	}

	return SQLToken{Type: TokenIdentifier, Start: start, End: l.pos}
}

func (l *sqllLexer) readOperator() SQLToken {
	start := l.pos
	ch := l.input[l.pos]
	l.pos++

	// Multi-char operators
	if ch == '<' && l.pos < len(l.input) {
		if l.input[l.pos] == '=' || l.input[l.pos] == '>' {
			l.pos++
		}
	} else if ch == '>' && l.pos < len(l.input) && l.input[l.pos] == '=' {
		l.pos++
	} else if ch == '!' && l.pos < len(l.input) && l.input[l.pos] == '=' {
		l.pos++
	} else if ch == ':' && l.pos < len(l.input) && l.input[l.pos] == '=' {
		l.pos++
	} else if ch == '|' && l.pos < len(l.input) && l.input[l.pos] == '|' {
		l.pos++
	}

	return SQLToken{Type: TokenOperator, Start: start, End: l.pos}
}

// --- Character classification ---

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch > 127
}

func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

func isOperator(ch rune) bool {
	switch ch {
	case '=', '<', '>', '!', '+', '-', '*', '/', '%', '|', '&', '^', '~':
		return true
	}
	return false
}

func isPunctuation(ch rune) bool {
	switch ch {
	case '(', ')', ',', ';', '.', '[', ']':
		return true
	}
	return false
}

// --- Color mapping ---

func colorForToken(t SQLTokenType) tcell.Color {
	switch t {
	case TokenKeyword:
		return tcell.ColorDodgerBlue
	case TokenString:
		return tcell.ColorOrange
	case TokenNumber:
		return tcell.ColorLimeGreen
	case TokenComment:
		return tcell.ColorGray
	case TokenFunction:
		return tcell.ColorMediumPurple
	case TokenOperator:
		return tcell.ColorDarkOrange
	case TokenTypeDef:
		return tcell.ColorDarkCyan
	case TokenBoolean:
		return tcell.ColorOrangeRed
	case TokenParameter:
		return tcell.ColorGold
	case TokenIdentifier:
		return tcell.ColorDefault
	case TokenPunctuation:
		return tcell.ColorDefault
	default:
		return tcell.ColorDefault
	}
}

// --- Keyword sets ---

var sqlKeywords = map[string]bool{
	"SELECT":      true,
	"FROM":        true,
	"WHERE":       true,
	"AND":         true,
	"OR":          true,
	"NOT":         true,
	"IN":          true,
	"IS":          true,
	"NULL":        true,
	"LIKE":        true,
	"BETWEEN":     true,
	"EXISTS":      true,
	"AS":          true,
	"ON":          true,
	"JOIN":        true,
	"INNER":       true,
	"LEFT":        true,
	"RIGHT":       true,
	"OUTER":       true,
	"CROSS":       true,
	"FULL":        true,
	"NATURAL":     true,
	"USING":       true,
	"INSERT":      true,
	"INTO":        true,
	"VALUES":      true,
	"UPDATE":      true,
	"SET":         true,
	"DELETE":      true,
	"CREATE":      true,
	"TABLE":       true,
	"INDEX":       true,
	"VIEW":        true,
	"PROCEDURE":   true,
	"FUNCTION":    true,
	"TRIGGER":     true,
	"ALTER":       true,
	"DROP":        true,
	"ADD":         true,
	"COLUMN":      true,
	"CONSTRAINT":  true,
	"PRIMARY":     true,
	"KEY":         true,
	"FOREIGN":     true,
	"UNIQUE":      true,
	"CHECK":       true,
	"DEFAULT":     true,
	"REFERENCES":  true,
	"CASCADE":     true,
	"ORDER":       true,
	"BY":          true,
	"ASC":         true,
	"DESC":        true,
	"GROUP":       true,
	"HAVING":      true,
	"LIMIT":       true,
	"OFFSET":      true,
	"UNION":       true,
	"ALL":         true,
	"INTERSECT":   true,
	"EXCEPT":      true,
	"DISTINCT":    true,
	"TOP":         true,
	"FETCH":       true,
	"NEXT":        true,
	"ROWS":        true,
	"ONLY":        true,
	"WITH":        true,
	"RECURSIVE":   true,
	"CASE":        true,
	"WHEN":        true,
	"THEN":        true,
	"ELSE":        true,
	"END":         true,
	"BEGIN":       true,
	"COMMIT":      true,
	"ROLLBACK":    true,
	"TRANSACTION": true,
	"SAVEPOINT":   true,
	"RELEASE":     true,
	"EXPLAIN":     true,
	"ANALYZE":     true,
	"DESCRIBE":    true,
	"SHOW":        true,
	"USE":         true,
	"GRANT":       true,
	"REVOKE":      true,
	"TRUNCATE":    true,
	"REPLACE":     true,
	"CALL":        true,
	"IF":          true,
	"ELSEIF":      true,
	"WHILE":       true,
	"LOOP":        true,
	"DECLARE":     true,
	"RETURN":      true,
	"DO":          true,
	"FOR":         true,
	"EACH":        true,
	"ROW":         true,
	"SCHEMA":      true,
	"DATABASE":    true,
	"TEMPORARY":   true,
	"TEMP":        true,
	"IFNULL":      true,
	"COALESCE":    true,
	"CAST":        true,
	"CONVERT":     true,
	"ANY":         true,
	"SOME":        true,
	"EXEC":        true,
	"EXECUTE":     true,
}

func isKeyword(upper string) bool {
	return sqlKeywords[upper]
}

var sqlTypes = map[string]bool{
	"INT":              true,
	"INTEGER":          true,
	"SMALLINT":         true,
	"BIGINT":           true,
	"TINYINT":          true,
	"MEDIUMINT":        true,
	"REAL":             true,
	"FLOAT":            true,
	"DOUBLE":           true,
	"DECIMAL":          true,
	"NUMERIC":          true,
	"CHAR":             true,
	"VARCHAR":          true,
	"TEXT":             true,
	"TINYTEXT":         true,
	"MEDIUMTEXT":       true,
	"LONGTEXT":         true,
	"BLOB":             true,
	"TINYBLOB":         true,
	"MEDIUMBLOB":       true,
	"LONGBLOB":         true,
	"BINARY":           true,
	"VARBINARY":        true,
	"BOOLEAN":          true,
	"BOOL":             true,
	"DATE":             true,
	"DATETIME":         true,
	"TIMESTAMP":        true,
	"TIME":             true,
	"YEAR":             true,
	"ENUM":             true,
	"SET":              true,
	"JSON":             true,
	"SERIAL":           true,
	"UUID":             true,
	"GEOMETRY":         true,
	"POINT":            true,
	"LINESTRING":       true,
	"POLYGON":          true,
	"INTERVAL":         true,
	"BYTEA":            true,
	"VARYING":          true,
	"CHARACTER":        true,
	"NVARCHAR":         true,
	"NCHAR":            true,
	"NTEXT":            true,
	"MONEY":            true,
	"SMALLMONEY":       true,
	"UNIQUEIDENTIFIER": true,
	"IMAGE":            true,
	"XML":              true,
	"CLOB":             true,
	"RAW":              true,
	"NUMBER":           true,
	"PLS_INTEGER":      true,
}

func isType(upper string) bool {
	return sqlTypes[upper]
}

var sqlBooleans = map[string]bool{
	"TRUE":    true,
	"FALSE":   true,
	"NULL":    true,
	"UNKNOWN": true,
}

func isBoolean(upper string) bool {
	return sqlBooleans[upper]
}

// visibleLen returns the display width of s, counting tabs as tabWidth spaces.
func visibleLen(s string, tabWidth int) int {
	w := 0
	for _, ch := range s {
		if ch == '\t' {
			w += tabWidth - (w % tabWidth)
		} else {
			w += runeWidth(ch)
		}
	}
	return w
}

func runeWidth(r rune) int {
	if r == '\t' || r == '\n' || r == '\r' {
		return 0
	}
	if r < 128 {
		return 1
	}
	// Simplified: treat all non-ASCII as width 2 (CJK etc.) or 1
	// For SQL purposes, this is almost always 1
	return 1
}
