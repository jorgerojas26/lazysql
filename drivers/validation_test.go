package drivers

import "testing"

func TestIsQueryMutation(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		// SELECT queries (should not be mutations)
		{"simple select", "SELECT * FROM users", false},
		{"select with whitespace", "  select id from table  ", false},
		{"select with joins", "SELECT u.* FROM users u JOIN orders o ON u.id = o.user_id", false},
		{"select with subquery", "SELECT * FROM (SELECT id FROM users) AS sub", false},

		// INSERT queries (should be mutations)
		{"simple insert", "INSERT INTO users VALUES (1)", true},
		{"insert select", "INSERT INTO users SELECT * FROM temp", true},
		{"insert with spaces", "  INSERT INTO table (col) VALUES (1)", true},
		{"insert with subquery", "INSERT INTO users (id, name) SELECT id, name FROM temp WHERE id IN (SELECT id FROM active)", true},

		// UPDATE queries (should be mutations)
		{"simple update", "UPDATE users SET name='test'", true},
		{"update with where", "UPDATE users SET name='test' WHERE id=1", true},
		{"update with subquery", "UPDATE products SET price = 100 WHERE product_id IN (SELECT product_id FROM discounted)", true},

		// DELETE queries (should be mutations)
		{"simple delete", "DELETE FROM users", true},
		{"delete with where", "DELETE FROM users WHERE id=1", true},
		{"delete with subquery", "DELETE FROM products WHERE product_id IN (SELECT product_id FROM products WHERE price < 10)", true},

		// DROP queries (should be mutations)
		{"drop table", "DROP TABLE users", true},
		{"drop database", "DROP DATABASE testdb", true},
		{"drop index", "DROP INDEX idx_name", true},

		// ALTER queries (should be mutations)
		{"alter table", "ALTER TABLE users ADD COLUMN email", true},
		{"alter database", "ALTER DATABASE testdb SET timezone='UTC'", true},

		// TRUNCATE queries (should be mutations)
		{"truncate", "TRUNCATE TABLE users", true},

		// CREATE queries (should be mutations, except temp)
		{"create table", "CREATE TABLE test (id int)", true},
		{"create view", "CREATE VIEW v AS SELECT 1", true},
		{"create index", "CREATE INDEX idx ON users(name)", true},
		{"create database", "CREATE DATABASE testdb", true},

		// Temporary objects (should NOT be mutations - allowed)
		{"create temp table", "CREATE TEMPORARY TABLE temp (id int)", false},
		{"create temp table short", "CREATE TEMP TABLE temp (id int)", false},
		{"create temp view", "CREATE TEMPORARY VIEW v AS SELECT 1", false},
		{"create temp view short", "CREATE TEMP VIEW v AS SELECT 1", false},

		// Comments (should ignore comments)
		{"select with single-line comment", "-- This is a comment\nSELECT * FROM users", false},
		{"select with multi-line comment", "/* This is a\n multi-line comment */ SELECT * FROM users", false},
		{"insert with comment", "-- comment\nINSERT INTO users VALUES (1)", true},
		{"commented out mutation", "-- INSERT INTO users\nSELECT * FROM users", false},

		// WITH clauses (CTE)
		{"with select", "WITH cte AS (SELECT 1) SELECT * FROM cte", false},
		{"with insert", "WITH cte AS (SELECT 1) INSERT INTO t SELECT * FROM cte", true},
		{"with update", "WITH cte AS (SELECT 1) UPDATE t SET x=1", true},

		// Other mutations
		{"replace", "REPLACE INTO users VALUES (1)", true},
		{"merge", "MERGE INTO users USING source ON condition", true},
		{"grant", "GRANT SELECT ON users TO user1", true},
		{"revoke", "REVOKE SELECT ON users FROM user1", true},
		{"rename", "RENAME TABLE old TO new", true},

		// Case variations
		{"lowercase insert", "insert into users values (1)", true},
		{"mixed case insert", "InSeRt InTo users VALUES (1)", true},
		{"uppercase select", "SELECT * FROM USERS", false},

		// Complex queries
		{"explain select", "EXPLAIN SELECT * FROM users", false},
		{"explain analyze select", "EXPLAIN ANALYZE SELECT * FROM users", false},
		{"show tables", "SHOW TABLES", false},
		{"describe table", "DESCRIBE users", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsQueryMutation(tt.query)
			if result != tt.expected {
				t.Errorf("IsQueryMutation(%q) = %v, want %v", tt.query, result, tt.expected)
			}
		})
	}
}

func TestValidateQueryForReadOnly(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		shouldErr bool
	}{
		{"select allowed", "SELECT * FROM users", false},
		{"insert blocked", "INSERT INTO users VALUES (1)", true},
		{"update blocked", "UPDATE users SET x=1", true},
		{"delete blocked", "DELETE FROM users", true},
		{"temp table allowed", "CREATE TEMP TABLE t (id int)", false},
		{"regular table blocked", "CREATE TABLE t (id int)", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateQueryForReadOnly(tt.query)
			if (err != nil) != tt.shouldErr {
				if tt.shouldErr {
					t.Errorf("ValidateQueryForReadOnly(%q) expected error, got nil", tt.query)
				} else {
					t.Errorf("ValidateQueryForReadOnly(%q) expected no error, got: %v", tt.query, err)
				}
			}
		})
	}
}
