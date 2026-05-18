package components

import (
	"strings"
	"unicode"
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

// GetCompletions returns completion items matching the given prefix.
// If tableHint is non-empty, it prioritises columns from that table.
func (a *Autocompleter) GetCompletions(prefix string, tableHint string) []CompletionItem {
	if prefix == "" {
		return nil
	}

	lower := strings.ToLower(prefix)
	var results []CompletionItem
	seen := make(map[string]bool)

	// Helper to deduplicate
	addIfMatch := func(items []CompletionItem) {
		for _, item := range items {
			itemLower := strings.ToLower(item.Text)
			if strings.HasPrefix(itemLower, lower) && !seen[itemLower] {
				results = append(results, item)
				seen[itemLower] = true
			}
		}
	}

	// 1. Columns from the hinted table (highest priority)
	if tableHint != "" {
		if cols, ok := a.columns[strings.ToLower(tableHint)]; ok {
			addIfMatch(cols)
		}
	}

	// 2. Table names
	addIfMatch(a.tables)

	// 3. Keywords (lower priority)
	addIfMatch(a.keywords)

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
	if cursorPos <= 0 || cursorPos > len(text) {
		return ""
	}

	// Find the start of the current word
	start := cursorPos - 1
	for start >= 0 {
		ch := rune(text[start])
		if unicode.IsSpace(ch) || isPunctuation(ch) {
			break
		}
		start--
	}
	start++

	if start >= cursorPos {
		return ""
	}

	return text[start:cursorPos]
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
