package drivers

import (
	"errors"
	"regexp"
	"strings"
)

// sql keywords that are blocked in read-only mode
var readOnlyBlockedKeywords = []string{
	"INSERT", "UPDATE", "DELETE", "DROP", "ALTER",
	"TRUNCATE", "REPLACE", "MERGE",
	"GRANT", "REVOKE", "RENAME",
	"CREATE TABLE", "CREATE INDEX", "CREATE DATABASE",
	"CREATE SCHEMA", "CREATE VIEW", "CREATE FUNCTION",
	"CREATE PROCEDURE", "CREATE TRIGGER",
}

// sql keywords that are allowed even in read-only mode
var readOnlyAllowedKeywords = []string{
	"CREATE TEMPORARY TABLE", "CREATE TEMP TABLE",
	"CREATE TEMP VIEW", "CREATE TEMPORARY VIEW",
}

// optional modifiers that may sit between CREATE and the object keyword
var createModifiersPattern = regexp.MustCompile(`^CREATE\s+(?:(?:OR\s+REPLACE|UNIQUE|MATERIALIZED)\s+)+`)

// StripSQLComments removes -- line comments and /* block */ comments from a query
func StripSQLComments(query string) string {
	// remove single-line comments (-- comment)
	query = regexp.MustCompile(`--[^\n]*`).ReplaceAllString(query, "")
	// remove multi-line comments (/* comment */)
	query = regexp.MustCompile(`/\*[\s\S]*?\*/`).ReplaceAllString(query, "")
	return strings.TrimSpace(query)
}

// IsQueryMutation checks if a SQL query is a mutation operation
func IsQueryMutation(query string) bool {
	upperQuery := StripSQLComments(strings.ToUpper(query))

	// normalize optional CREATE modifiers so the prefix checks below still
	// match, for example "CREATE OR REPLACE VIEW" or "CREATE UNIQUE INDEX"
	upperQuery = createModifiersPattern.ReplaceAllString(upperQuery, "CREATE ")

	// check if query starts with allowed keywords
	for _, allowedKeyword := range readOnlyAllowedKeywords {
		if strings.HasPrefix(upperQuery, allowedKeyword) {
			return false
		}
	}

	// check for blocked mutation keywords
	for _, blockedKeyword := range readOnlyBlockedKeywords {
		// check if query starts with the keyword
		if strings.HasPrefix(upperQuery, blockedKeyword) {
			return true
		}
		// check for WITH clause followed by mutation
		// an example - "WITH cte AS (SELECT 1) INSERT INTO ..."
		// (?s) makes . match newlines so multi-line CTEs are covered too
		// the trailing \b keeps identifiers such as "UPDATED_AT" from matching
		withPattern := `(?s)^WITH\s+.*\s+` + regexp.QuoteMeta(blockedKeyword) + `\b`
		if matched, _ := regexp.MatchString(withPattern, upperQuery); matched {
			return true
		}
	}

	return false
}

func ValidateQueryForReadOnly(query string) error {
	if IsQueryMutation(query) {
		return errors.New("mutation queries are not allowed in read-only mode")
	}
	return nil
}
