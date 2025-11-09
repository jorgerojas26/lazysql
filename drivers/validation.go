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

// IsQueryMutation checks if a SQL query is a mutation operation
func IsQueryMutation(query string) bool {
	upperQuery := strings.TrimSpace(strings.ToUpper(query))

	// remove single-line comments (-- comment)
	upperQuery = regexp.MustCompile(`--[^\n]*`).ReplaceAllString(upperQuery, "")
	// remove multi-line comments (/* comment */)
	upperQuery = regexp.MustCompile(`/\*[\s\S]*?\*/`).ReplaceAllString(upperQuery, "")
	upperQuery = strings.TrimSpace(upperQuery)

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
		withPattern := `^WITH\s+.*\s+` + regexp.QuoteMeta(blockedKeyword)
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
