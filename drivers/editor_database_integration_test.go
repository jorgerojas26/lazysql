package drivers

import (
	"context"
	"os"
	"testing"
)

// Set these URLs to disposable instances: this test creates two databases.
func TestEditorDatabaseIntegration(t *testing.T) {
	for _, provider := range []string{"MYSQL", "MSSQL", "POSTGRES"} {
		t.Run(provider, func(t *testing.T) {
			url := os.Getenv("LAZYSQL_TEST_" + provider + "_URL")
			if url == "" {
				t.Skip("disposable database URL not set")
			}
			var db Driver
			var quote func(string) string
			switch provider {
			case "MYSQL":
				db = &MySQL{}
				quote = func(s string) string { return "`" + s + "`" }
			case "MSSQL":
				db = &MSSQL{}
				quote = quoteMSSQLIdentifier
			case "POSTGRES":
				db = &Postgres{}
				quote = func(s string) string { return `"` + s + `"` }
			}
			if err := db.Connect(context.Background(), url); err != nil {
				t.Fatal(err)
			}
			switch d := db.(type) {
			case *MySQL:
				d.Connection.SetMaxOpenConns(1)
				t.Cleanup(func() { _ = d.Connection.Close() })
			case *MSSQL:
				d.Connection.SetMaxOpenConns(1)
				t.Cleanup(func() { _ = d.Connection.Close() })
			case *Postgres:
				d.Connection.SetMaxOpenConns(1)
				t.Cleanup(func() { _ = d.Connection.Close() })
			}
			exec := func(database, query string) {
				t.Helper()
				if _, err := db.ExecuteDMLStatement(context.Background(), database, query); err != nil {
					t.Fatalf("%s: %v", query, err)
				}
			}
			for _, database := range []string{"pr330_a", "pr330-b"} {
				exec("", "CREATE DATABASE "+quote(database))
				t.Cleanup(func() { _, _ = db.ExecuteDMLStatement(context.Background(), "", "DROP DATABASE "+quote(database)) })
				exec(database, "CREATE TABLE items (value INT)")
				exec(database, "INSERT INTO items VALUES (1)")
			}
			exec("pr330-b", "UPDATE items SET value = 2")
			check := func(database, query, want string) {
				t.Helper()
				rows, n, err := db.ExecuteQuery(context.Background(), database, query)
				if err != nil || n != 1 || len(rows) != 2 || rows[1][0] != want {
					t.Fatalf("database %q: got %v (%d), %v; want %q", database, rows, n, err, want)
				}
			}
			streamCheck := func(database, want string) {
				t.Helper()
				var rows [][]string
				result, err := db.(QueryStreamer).StreamQuery(context.Background(), database, "SELECT value FROM items", 1, func(batch QueryBatch) error { rows = append(rows, batch.Rows...); return nil })
				if err != nil || result.Rows != 1 || result.Truncated || len(rows) != 1 || rows[0][0] != want {
					t.Fatalf("stream database %q: got %v, %+v, %v; want %s", database, rows, result, err, want)
				}
			}
			for range 3 {
				streamCheck("pr330-b", "2")
				streamCheck("pr330_a", "1")
				check("pr330-b", "SELECT value FROM items", "2")
				check("pr330_a", "SELECT value FROM items", "1")
				if _, _, err := db.ExecuteQuery(context.Background(), "pr330-b", "SELECT * FROM missing_table"); err == nil {
					t.Fatal("expected query failure")
				}
				current := "SELECT DATABASE()"
				if provider == "MSSQL" {
					current = "SELECT DB_NAME()"
				}
				if provider == "POSTGRES" {
					current = "SELECT current_database()"
				}
				check("", current, "original")
				check("original", current, "original")
			}
			if provider == "MSSQL" {
				// CREATE VIEW must be first in its batch: a USE prefix breaks it.
				exec("pr330-b", "CREATE VIEW test_view AS SELECT value FROM items")
				check("pr330-b", "SELECT value FROM test_view", "2")
			}
		})
	}
}
