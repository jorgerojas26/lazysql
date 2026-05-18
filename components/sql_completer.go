package components

import (
	"sort"
	"strings"
	"unicode"

	"github.com/lithammer/fuzzysearch/fuzzy"
)

// CompletionItem represents a single autocomplete suggestion.
type CompletionItem struct {
	Text        string // what gets inserted
	Description string // displayed help text
}

// Autocompleter manages SQL keywords and schema-aware completions.
type Autocompleter struct {
	keywords []CompletionItem
	tables   []CompletionItem
	columns  map[string][]CompletionItem // table name -> columns
}

// NewAutocompleter creates an autocompleter with built-in SQL keywords.
func NewAutocompleter() *Autocompleter {
	return &Autocompleter{
		keywords: builtinKeywords(),
		tables:   nil,
		columns:  make(map[string][]CompletionItem),
	}
}

// SetTables updates the list of known table names.
func (a *Autocompleter) SetTables(tables []string) {
	a.tables = make([]CompletionItem, len(tables))
	for i, t := range tables {
		a.tables[i] = CompletionItem{Text: t, Description: "table"}
	}
}

// SetColumns sets the columns for a given table.
func (a *Autocompleter) SetColumns(table string, columns []string) {
	items := make([]CompletionItem, len(columns))
	for i, c := range columns {
		items[i] = CompletionItem{Text: c, Description: "column"}
	}
	a.columns[strings.ToLower(table)] = items
}

// GetCompletions returns completion items matching the given prefix using
// fuzzy search (same ranking as the tree: exact > prefix > substring > fuzzy).
// If tableHint is non-empty, it prioritises columns from that table.
// When prefix is empty but tableHint is set (e.g. user typed "table."), all
// columns for that table are returned.
func (a *Autocompleter) GetCompletions(prefix string, tableHint string) []CompletionItem {
	// When prefix is empty but a table is specified, show all columns for that table.
	if prefix == "" && tableHint != "" {
		if cols, ok := a.columns[strings.ToLower(tableHint)]; ok {
			return cols
		}
		return nil
	}
	if prefix == "" {
		return nil
	}

	type scoredCandidate struct {
		item  CompletionItem
		score int    // lower = better match (0=exact, 1-99=prefix, 100+=substr/fuzzy)
		order int    // priority group (0=column, 1=table, 2=keyword)
	}

	var candidates []scoredCandidate
	seen := make(map[string]bool)
	lowerPrefix := strings.ToLower(prefix)

	// Collect candidates with a score and dedup
	tryAdd := func(items []CompletionItem, order int) {
		for _, item := range items {
			key := strings.ToLower(item.Text)
			if seen[key] {
				continue
			}
			// Use the same prioritization as the tree search
			rank := fuzzy.RankMatch(lowerPrefix, key)
			if rank < 0 {
				continue
			}
			score := prioritizeResult(lowerPrefix, key, rank)
			candidates = append(candidates, scoredCandidate{item, score, order})
			seen[key] = true
		}
	}

	// 1. Columns from the hinted table (highest priority)
	if tableHint != "" {
		if cols, ok := a.columns[strings.ToLower(tableHint)]; ok {
			tryAdd(cols, 0)
		}
	}

	// 2. Table names
	tryAdd(a.tables, 1)

	// 3. Keywords (lower priority)
	tryAdd(a.keywords, 2)

	// Sort by score ascending (lower = better), then by priority group
	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].score != candidates[j].score {
			return candidates[i].score < candidates[j].score
		}
		return candidates[i].order < candidates[j].order
	})

	// Limit to top 20 results
	maxResults := 20
	if len(candidates) < maxResults {
		maxResults = len(candidates)
	}

	results := make([]CompletionItem, maxResults)
	for i := 0; i < maxResults; i++ {
		results[i] = candidates[i].item
	}

	return results
}

// GetAllCompletions returns all known keywords, tables, and column names
// matching the prefix, in order of relevance.
func (a *Autocompleter) GetAllCompletions(prefix string) []CompletionItem {
	return a.GetCompletions(prefix, "")
}

// extractPrefix extracts the word being typed at the cursor position.
// text is the full text, cursorPos is the byte offset of the cursor.
func extractPrefix(text string, cursorPos int) string {
	prefix, _ := extractCompletionContext(text, cursorPos)
	return prefix
}

// extractCompletionContext splits the current word segment at the cursor into
// a column prefix and an optional table name. For "table.col|" it returns
// ("col", "table"). For "prefix|" alone it returns ("prefix", "").
func extractCompletionContext(text string, cursorPos int) (prefix, tableName string) {
	if cursorPos <= 0 || cursorPos > len(text) {
		return "", ""
	}

	// Walk backward from cursor to find the start of the current token.
	// Unlike extractPrefix, we do NOT stop at '.' — we want table.col intact.
	start := cursorPos - 1
	for start >= 0 {
		ch := rune(text[start])
		if unicode.IsSpace(ch) || ch == ';' || ch == ',' || ch == '(' || ch == ')' {
			break
		}
		start--
	}
	start++

	if start >= cursorPos {
		return "", ""
	}

	segment := text[start:cursorPos]

	// Check for table.col pattern (last dot in the segment)
	if dotIdx := strings.LastIndex(segment, "."); dotIdx >= 0 {
		tableName = segment[:dotIdx]
		prefix = segment[dotIdx+1:]
		return prefix, tableName
	}

	// Regular prefix without a dot
	return segment, ""
}

// resolveAliases scans the query text for table alias definitions in
// FROM/JOIN clauses and returns a map of alias → table name.
// Handles "FROM table alias", "FROM table AS alias", "JOIN table alias".
func resolveAliases(sql string) map[string]string {
	aliases := make(map[string]string)
	upper := strings.ToUpper(sql)

	keywords := []string{"FROM ", "JOIN ", "INTO ", "UPDATE "}
	for _, kw := range keywords {
		searchFrom := 0
		for {
			pos := strings.Index(upper[searchFrom:], kw)
			if pos < 0 {
				break
			}
			pos += searchFrom + len(kw)

			// Skip whitespace
			for pos < len(sql) && (sql[pos] == ' ' || sql[pos] == '\t') {
				pos++
			}

			// Read table name
			tableStart := pos
			for pos < len(sql) && !unicode.IsSpace(rune(sql[pos])) && sql[pos] != ',' && sql[pos] != ';' && sql[pos] != '(' && sql[pos] != ')' {
				pos++
			}
			tableName := sql[tableStart:pos]
			if tableName == "" {
				searchFrom = pos
				continue
			}

			// Skip whitespace after table name
			for pos < len(sql) && (sql[pos] == ' ' || sql[pos] == '\t') {
				pos++
			}

			// Skip optional AS keyword
			if pos+2 <= len(sql) && strings.ToUpper(sql[pos:pos+2]) == "AS" {
				pos += 2
				for pos < len(sql) && (sql[pos] == ' ' || sql[pos] == '\t') {
					pos++
				}
			}

			// Read alias name (single identifier, not a keyword)
			aliasStart := pos
			for pos < len(sql) && !unicode.IsSpace(rune(sql[pos])) && sql[pos] != ',' && sql[pos] != ';' && sql[pos] != '(' && sql[pos] != ')' && sql[pos] != '=' && sql[pos] != '\n' && sql[pos] != '\r' {
				pos++
			}
			alias := sql[aliasStart:pos]
			if alias != "" && alias != tableName {
				aliases[strings.ToLower(alias)] = tableName
			}

			searchFrom = pos
		}
	}
	return aliases
}

// extractTableHint tries to guess which table the user is referencing
// based on the text before the cursor. Very simple heuristic: looks for
// "FROM <word>" or "JOIN <word>" or "<word>." preceding the cursor.
func extractTableHint(text string, cursorPos int) string {
	if cursorPos > len(text) {
		cursorPos = len(text)
	}

	// Check for "tablename." before cursor
	before := text[:cursorPos]
	if idx := strings.LastIndex(before, "."); idx > 0 {
		// Find the identifier before the dot
		start := idx - 1
		for start >= 0 {
			ch := rune(before[start])
			if unicode.IsSpace(ch) || isPunctuation(ch) {
				break
			}
			start--
		}
		start++
		if start < idx {
			return before[start:idx]
		}
	}

	// Look for FROM or JOIN keywords
	upper := strings.ToUpper(before)
	lastFrom := strings.LastIndex(upper, "FROM ")
	lastJoin := strings.LastIndex(upper, "JOIN ")
	lastInto := strings.LastIndex(upper, "INTO ")

	best := -1
	keywordLen := 0

	for _, kw := range []struct {
		idx int
		str string
	}{
		{lastFrom, "FROM "},
		{lastJoin, "JOIN "},
		{lastInto, "INTO "},
	} {
		if kw.idx > best {
			best = kw.idx
			keywordLen = len(kw.str)
		}
	}

	if best >= 0 {
		after := before[best+keywordLen:]
		// Extract the next word
		after = strings.TrimLeft(after, " \t")
		end := strings.IndexAny(after, " \t\n\r,;")
		if end > 0 {
			return after[:end]
		} else if end == -1 && after != "" {
			return after
		}
	}

	return ""
}

// builtinKeywords returns the list of SQL keywords for autocomplete.
func builtinKeywords() []CompletionItem {
	return []CompletionItem{
		{Text: "SELECT", Description: "Retrieve rows from a table"},
		{Text: "FROM", Description: "Specify the source table"},
		{Text: "WHERE", Description: "Filter results"},
		{Text: "AND", Description: "Logical AND"},
		{Text: "OR", Description: "Logical OR"},
		{Text: "NOT", Description: "Logical NOT"},
		{Text: "IN", Description: "Check membership"},
		{Text: "IS", Description: "Null comparison"},
		{Text: "NULL", Description: "Null value"},
		{Text: "LIKE", Description: "Pattern matching"},
		{Text: "BETWEEN", Description: "Range check"},
		{Text: "EXISTS", Description: "Check existence"},
		{Text: "AS", Description: "Alias"},
		{Text: "ON", Description: "Join condition"},
		{Text: "JOIN", Description: "Join tables"},
		{Text: "INNER", Description: "Inner join"},
		{Text: "LEFT", Description: "Left join"},
		{Text: "RIGHT", Description: "Right join"},
		{Text: "OUTER", Description: "Outer join"},
		{Text: "CROSS", Description: "Cross join"},
		{Text: "FULL", Description: "Full join"},
		{Text: "NATURAL", Description: "Natural join"},
		{Text: "USING", Description: "Join using columns"},
		{Text: "INSERT", Description: "Insert rows"},
		{Text: "INTO", Description: "Specify target table"},
		{Text: "VALUES", Description: "Row values"},
		{Text: "UPDATE", Description: "Update rows"},
		{Text: "SET", Description: "Set column values"},
		{Text: "DELETE", Description: "Delete rows"},
		{Text: "CREATE", Description: "Create object"},
		{Text: "TABLE", Description: "Create table"},
		{Text: "INDEX", Description: "Create index"},
		{Text: "VIEW", Description: "Create view"},
		{Text: "ALTER", Description: "Modify object"},
		{Text: "DROP", Description: "Delete object"},
		{Text: "ADD", Description: "Add column"},
		{Text: "COLUMN", Description: "Column keyword"},
		{Text: "CONSTRAINT", Description: "Add constraint"},
		{Text: "PRIMARY KEY", Description: "Primary key"},
		{Text: "FOREIGN KEY", Description: "Foreign key"},
		{Text: "UNIQUE", Description: "Unique constraint"},
		{Text: "CHECK", Description: "Check constraint"},
		{Text: "DEFAULT", Description: "Default value"},
		{Text: "REFERENCES", Description: "Foreign key reference"},
		{Text: "ORDER BY", Description: "Order results"},
		{Text: "ASC", Description: "Ascending order"},
		{Text: "DESC", Description: "Descending order"},
		{Text: "GROUP BY", Description: "Group results"},
		{Text: "HAVING", Description: "Filter groups"},
		{Text: "LIMIT", Description: "Limit rows"},
		{Text: "OFFSET", Description: "Offset rows"},
		{Text: "UNION", Description: "Combine queries"},
		{Text: "ALL", Description: "All results"},
		{Text: "DISTINCT", Description: "Distinct rows"},
		{Text: "CASE", Description: "Case expression"},
		{Text: "WHEN", Description: "Case when"},
		{Text: "THEN", Description: "Case then"},
		{Text: "ELSE", Description: "Case else"},
		{Text: "END", Description: "End expression"},
		{Text: "BEGIN", Description: "Start transaction"},
		{Text: "COMMIT", Description: "Commit transaction"},
		{Text: "ROLLBACK", Description: "Rollback transaction"},
		{Text: "EXPLAIN", Description: "Explain query plan"},
		{Text: "DESCRIBE", Description: "Describe table"},
		{Text: "SHOW", Description: "Show objects"},
		{Text: "USE", Description: "Use database"},
		{Text: "TRUNCATE", Description: "Truncate table"},
		{Text: "REPLACE", Description: "Replace rows"},
		{Text: "CALL", Description: "Call procedure"},
		{Text: "WITH", Description: "Common table expression"},
		{Text: "RECURSIVE", Description: "Recursive CTE"},
		{Text: "FETCH", Description: "Fetch rows"},
		{Text: "NEXT", Description: "Fetch next"},
		{Text: "ROWS", Description: "Row count"},
		{Text: "ONLY", Description: "Fetch only"},
		{Text: "TOP", Description: "Select top"},
		{Text: "COUNT", Description: "Count rows"},
		{Text: "SUM", Description: "Sum values"},
		{Text: "AVG", Description: "Average values"},
		{Text: "MIN", Description: "Minimum value"},
		{Text: "MAX", Description: "Maximum value"},
		{Text: "COALESCE", Description: "First non-null"},
		{Text: "IFNULL", Description: "Null fallback"},
		{Text: "CAST", Description: "Type cast"},
		{Text: "CONVERT", Description: "Type convert"},
		{Text: "CROSS JOIN", Description: "Cross join"},
		{Text: "NATURAL JOIN", Description: "Natural join"},
		{Text: "INNER JOIN", Description: "Inner join"},
		{Text: "LEFT JOIN", Description: "Left join"},
		{Text: "RIGHT JOIN", Description: "Right join"},
		{Text: "FULL JOIN", Description: "Full join"},
		{Text: "OUTER JOIN", Description: "Outer join"},
		{Text: "ASC", Description: "Ascending"},
		{Text: "DESC", Description: "Descending"},
		{Text: "COUNT(*)", Description: "Count all rows"},
		{Text: "DISTINCT", Description: "Unique values only"},
	}
}
