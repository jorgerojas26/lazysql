package components

import (
	"strings"
)

// ---------------------------------------------------------------------------
// SQLContext – structured model built from the lexer token stream
// ---------------------------------------------------------------------------

// SQLContext holds everything the autocomplete needs: known aliases, table
// names, CTEs, and per-depth table references so we can answer "what table
// is visible near the cursor?".
type SQLContext struct {
	Aliases map[string]string // alias → resolved table name (lowercase keys)
	Tables  []string          // all unique table names in order of appearance
	CTEs    map[string]bool   // CTE names (also appear in Aliases)

	// tableRefs keeps every reference with its depth & byte offset so we can
	// find the most recently mentioned table at the cursor's nesting level.
	tableRefs []tableRef
}

type tableRef struct {
	name  string
	alias string // empty when the table has no alias
	depth int    // subquery nesting depth (0 = main query)
	pos   int    // byte offset in the original SQL text
}

// scanSQLContext tokenises sql and walks the token stream once to build a
// structured context model.  It understands:
//   - Subquery nesting (parentheses depth)
//   - CTE declarations (WITH … AS)
//   - Comma-separated table lists
//   - Subqueries in FROM/JOIN clauses
//   - Schema-qualified names (schema.table)
//
// The old string-search-based helpers were replaced by this because they
// could not distinguish outer FROM from inner FROM inside a subquery.
func scanSQLContext(text string) *SQLContext {
	tokens := tokenize(text)
	ctx := &SQLContext{
		Aliases: make(map[string]string),
		CTEs:    make(map[string]bool),
	}

	depth := 0
	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		word := text[tok.Start:tok.End]

		// Track parentheses depth.
		switch word {
		case "(":
			depth++
			i++
			continue
		case ")":
			depth--
			i++
			continue
		}

		// Only process keywords at valid depth.
		if tok.Type == TokenKeyword && depth >= 0 {
			upper := strings.ToUpper(word)
			switch upper {
			case "FROM", "JOIN", "INTO", "UPDATE":
				refs := readTableRefs(text, tokens, &i, depth)
				ctx.tableRefs = append(ctx.tableRefs, refs...)
				for _, ref := range refs {
					if ref.alias != "" {
						ctx.Aliases[strings.ToLower(ref.alias)] = ref.name
					}
				}
				continue // i already advanced past table list
			case "WITH":
				if depth == 0 {
					readCTEs(text, tokens, &i, ctx)
					continue
				}
			}
		}

		i++
	}

	// Build Tables slice preserving insertion order.
	seen := make(map[string]bool)
	for _, ref := range ctx.tableRefs {
		lower := strings.ToLower(ref.name)
		if !seen[lower] {
			ctx.Tables = append(ctx.Tables, ref.name)
			seen[lower] = true
		}
	}

	return ctx
}

// cursorDepth returns the subquery nesting depth at cursorPos by walking
// tokens and counting ( / ) up to that byte offset.
func cursorDepth(text string, cursorPos int) int {
	tokens := tokenize(text)
	depth := 0
	for _, t := range tokens {
		if t.Start >= cursorPos {
			break
		}
		w := text[t.Start:t.End]
		if w == "(" {
			depth++
		} else if w == ")" {
			depth--
		}
	}
	if depth < 0 {
		depth = 0
	}
	return depth
}

// ---------------------------------------------------------------------------
// Token-walking helpers
// ---------------------------------------------------------------------------

// clauseBoundary words stop a FROM/JOIN table list.
var clauseBoundaries = map[string]bool{
	"WHERE": true, "GROUP": true, "ORDER": true, "HAVING": true,
	"LIMIT": true, "OFFSET": true,
	"UNION": true, "INTERSECT": true, "EXCEPT": true,
	"RETURNING": true, "VALUES": true, "SET": true,
}

// joinModifiers are skipped when encountered right before JOIN.
var joinModifiers = map[string]bool{
	"INNER": true, "LEFT": true, "RIGHT": true, "CROSS": true,
	"FULL": true, "OUTER": true, "NATURAL": true, "LATERAL": true,
}

// aliasFollowers – when the token after a potential alias is one of these,
// the potential alias is considered a real alias.
var aliasFollowers = map[string]bool{
	",": true, "ON": true, "USING": true,
	"WHERE": true, "ORDER": true, "GROUP": true, "HAVING": true,
	"LIMIT": true, "OFFSET": true, "RETURNING": true,
	"JOIN": true, "INNER": true, "LEFT": true, "RIGHT": true,
	"CROSS": true, "FULL": true, "OUTER": true, "NATURAL": true,
	"UNION": true, "INTERSECT": true, "EXCEPT": true,
	")": true, ";": true, "END": true,
	"(": true, "SET": true,
	// "(" covers: INSERT INTO logs l (message) ...
	// "SET" covers: UPDATE accounts a SET balance = 0 ...
}

func isClauseBoundary(word string) bool {
	return clauseBoundaries[strings.ToUpper(word)]
}

func isJoinModifier(word string) bool {
	return joinModifiers[strings.ToUpper(word)]
}

func isAliasFollower(word string) bool {
	return aliasFollowers[strings.ToUpper(word)]
}

// skipWhitespace advances i past whitespace tokens.
func skipWhitespace(tokens []SQLToken, i *int) {
	for *i < len(tokens) && tokens[*i].Type == TokenWhitespace {
		*i++
	}
}

// readQualifiedName reads a possibly schema/db-qualified name starting at *i.
// It advances *i past the last consumed token.
func readQualifiedName(text string, tokens []SQLToken, i *int) string {
	name := text[tokens[*i].Start:tokens[*i].End]
	*i++

	// db.schema.table or schema.table
	for *i < len(tokens) && text[tokens[*i].Start:tokens[*i].End] == "." {
		*i++ // skip dot
		if *i < len(tokens) && (tokens[*i].Type == TokenIdentifier || tokens[*i].Type == TokenKeyword) {
			name += "." + text[tokens[*i].Start:tokens[*i].End]
			*i++
		} else {
			break
		}
	}
	return name
}

// skipParenBlock skips from the current '(' to its matching ')'.
// *i must point to the opening '(' token.  On return *i points to the
// token just after the matching ')'.
func skipParenBlock(text string, tokens []SQLToken, i *int) {
	if *i >= len(tokens) || text[tokens[*i].Start:tokens[*i].End] != "(" {
		return
	}
	depth := 1
	*i++
	for *i < len(tokens) && depth > 0 {
		w := text[tokens[*i].Start:tokens[*i].End]
		if w == "(" {
			depth++
		} else if w == ")" {
			depth--
		}
		*i++
	}
}

func isFromOrJoin(word string) bool {
	switch strings.ToUpper(word) {
	case "FROM", "JOIN", "INTO", "UPDATE":
		return true
	}
	return false
}

// scanParenForTableRefs scans tokens inside a parenthesised block (starting
// at *i which points to the opening '(') for nested FROM/JOIN keywords and
// records every table reference found at baseDepth+1 (and deeper).
// On return *i points to the token just after the matching ')'.
func scanParenForTableRefs(text string, tokens []SQLToken, i *int, baseDepth int, refs *[]tableRef) {
	if *i >= len(tokens) || text[tokens[*i].Start:tokens[*i].End] != "(" {
		return
	}
	innerDepth := 1
	*i++
	for *i < len(tokens) && innerDepth > 0 {
		tok := tokens[*i]
		w := text[tok.Start:tok.End]

		if w == "(" {
			innerDepth++
		} else if w == ")" {
			innerDepth--
			if innerDepth == 0 {
				*i++ // skip past ')'
				break
			}
		}

		if tok.Type == TokenKeyword && innerDepth >= 0 {
			if isFromOrJoin(w) {
				innerRefs := readTableRefs(text, tokens, i, baseDepth+1)
				*refs = append(*refs, innerRefs...)
				continue // readTableRefs already advanced *i
			}
		}

		*i++
	}
}

// readTableRefs reads table references starting after FROM/JOIN/INTO/UPDATE.
// It handles aliases, comma-separated lists, subqueries as table sources,
// schema-qualified names, and stops at clause boundaries.
//
// On return *i points to the first token after the table list.
func readTableRefs(text string, tokens []SQLToken, i *int, baseDepth int) []tableRef {
	*i++ // skip the introductory keyword (FROM/JOIN/INTO/UPDATE)
	var refs []tableRef

loop:
	for *i < len(tokens) {
		skipWhitespace(tokens, i)
		if *i >= len(tokens) {
			break
		}

		tok := tokens[*i]
		word := text[tok.Start:tok.End]

		// Clause boundaries end the list.
		if isClauseBoundary(word) {
			break
		}

		// Skip JOIN qualifiers (INNER, LEFT, CROSS, …).
		if isJoinModifier(word) {
			*i++
			continue
		}

		// Comma – next table in the list.
		if word == "," {
			*i++
			continue
		}

		// Parenthesized subquery:  FROM (SELECT …) AS alias
		if word == "(" {
			// Recursively scan inside the paren for FROM/JOIN keywords so
			// table refs INSIDE subqueries are found (cursor in subquery
			// should offer the inner tables).
			scanParenForTableRefs(text, tokens, i, baseDepth, &refs)

			// Optional AS alias after the subquery.
			skipWhitespace(tokens, i)
			if *i < len(tokens) && strings.EqualFold(text[tokens[*i].Start:tokens[*i].End], "AS") {
				*i++
				skipWhitespace(tokens, i)
			}
			if *i < len(tokens) && (tokens[*i].Type == TokenIdentifier || tokens[*i].Type == TokenKeyword) {
				alias := text[tokens[*i].Start:tokens[*i].End]
				// Register the subquery-derived table name with a self-alias so
				// resolveAliases can resolve "sub.col" even though "sub" is not
				// a real table (it points to itself).
				refs = append(refs, tableRef{name: alias, alias: alias, depth: baseDepth, pos: tokens[*i].Start})
				*i++
			}

			// Comma after a subquery means another table follows.
			skipWhitespace(tokens, i)
			if *i < len(tokens) && text[tokens[*i].Start:tokens[*i].End] == "," {
				*i++
				continue
			}
			break
		}

		// Regular table name (identifier or keyword used as name).
		if tok.Type == TokenIdentifier || tok.Type == TokenKeyword {
			name := readQualifiedName(text, tokens, i)
			alias := ""
			skipWhitespace(tokens, i)

			// ---- alias detection ----
			if *i < len(tokens) {
				nextTok := tokens[*i]
				nextWord := text[nextTok.Start:nextTok.End]

				// Explicit AS alias.
				if strings.EqualFold(nextWord, "AS") {
					*i++
					skipWhitespace(tokens, i)
					if *i < len(tokens) && (tokens[*i].Type == TokenIdentifier || tokens[*i].Type == TokenKeyword) {
						alias = text[tokens[*i].Start:tokens[*i].End]
						*i++
					}
				} else if (nextTok.Type == TokenIdentifier || nextTok.Type == TokenKeyword) &&
					!isClauseBoundary(nextWord) && !isJoinModifier(nextWord) && nextWord != "," && nextWord != "(" {
					// Potential implicit alias – peek ahead to confirm.
					save := *i
					*i++
					skipWhitespace(tokens, i)
					if *i >= len(tokens) || isAliasFollower(text[tokens[*i].Start:tokens[*i].End]) {
						alias = nextWord
						// *i already advanced past the alias.
					} else {
						// Not an alias – restore position.
						*i = save
					}
				}
			}

			refs = append(refs, tableRef{
				name:  name,
				alias: alias,
				depth: baseDepth,
				pos:   tok.Start,
			})

			// If an alias was resolved, also register a self-alias so that
			// e.g. "FROM users u" maps alias "u" → "users".
			if alias != "" {
				// The actual alias→table registration happens in scanSQLContext.
			}

			// Comma-separated continuation?
			skipWhitespace(tokens, i)
			if *i < len(tokens) && text[tokens[*i].Start:tokens[*i].End] == "," {
				*i++
				continue
			}

			// Table list ended.
			break loop
		}

		// Anything else ends the list.
		break
	}

	return refs
}

// readCTEs processes WITH … AS ( … ) definitions at depth 0.
// It registers CTE names in ctx.CTEs and ctx.Aliases, then skips past the
// CTE bodies so the main loop can continue scanning the main query.
func readCTEs(text string, tokens []SQLToken, i *int, ctx *SQLContext) {
	*i++ // skip WITH

	// Optional RECURSIVE.
	skipWhitespace(tokens, i)
	if *i < len(tokens) && strings.EqualFold(text[tokens[*i].Start:tokens[*i].End], "RECURSIVE") {
		*i++
	}

	for *i < len(tokens) {
		skipWhitespace(tokens, i)
		if *i >= len(tokens) {
			break
		}

		// CTE name.
		// Accept TokenFunction too:  t(n)  has no space before ( so the
		// lexer classifies 't' as TokenFunction (function-call detection).
		tok := tokens[*i]
		if tok.Type != TokenIdentifier && tok.Type != TokenKeyword && tok.Type != TokenFunction {
			// Not a CTE – main query started.
			break
		}
		name := text[tok.Start:tok.End]
		*i++

		// Skip optional column list: (col1, col2, …) at the current depth.
		// The pattern is: cte_name [(col_list)] AS (body)
		skipWhitespace(tokens, i)
		if *i < len(tokens) && text[tokens[*i].Start:tokens[*i].End] == "(" {
			skipParenBlock(text, tokens, i)
		}

		// Expect AS.
		skipWhitespace(tokens, i)
		if *i < len(tokens) && strings.EqualFold(text[tokens[*i].Start:tokens[*i].End], "AS") {
			*i++
		} else {
			// No AS — not a well-formed CTE, bail out.
			break
		}

		// Skip the subquery body ( … ), which may be nested.
		skipWhitespace(tokens, i)
		if *i < len(tokens) && text[tokens[*i].Start:tokens[*i].End] == "(" {
			skipParenBlock(text, tokens, i)
		}

		// Register the CTE.
		lower := strings.ToLower(name)
		ctx.CTEs[lower] = true
		ctx.Aliases[lower] = name

		// Comma-separated CTEs?
		skipWhitespace(tokens, i)
		if *i < len(tokens) && text[tokens[*i].Start:tokens[*i].End] == "," {
			*i++
			continue
		}
		break
	}
}
