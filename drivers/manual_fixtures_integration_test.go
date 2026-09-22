package drivers

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/jorgerojas26/lazysql/models"
)

// TestManualFixtures exercises real server result decoding, catalog queries,
// bounded pages and streaming. PostgreSQL routing uses disposable schemas.
// Run scripts/manual-databases.sh up first, then LAZYSQL_MANUAL_FIXTURES=1 go test ./drivers -run TestManualFixtures.
func TestManualFixtures(t *testing.T) {
	if os.Getenv("LAZYSQL_MANUAL_FIXTURES") != "1" {
		t.Skip("LAZYSQL_MANUAL_FIXTURES is not enabled")
	}
	var config struct {
		Databases []models.Connection `toml:"database"`
	}
	if _, err := toml.DecodeFile("../.lazysql.toml", &config); err != nil {
		t.Fatal(err)
	}
	if len(config.Databases) != 4 {
		t.Fatal("expected four manual fixtures")
	}
	for _, connection := range config.Databases {
		t.Run(connection.Provider, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			var db Driver
			var pool *sql.DB
			table, queryTable := "orders", "orders"
			switch connection.Provider {
			case DriverMySQL:
				db = &MySQL{}
			case DriverPostgres:
				db = &Postgres{}
				table = "public.orders"
				queryTable = table
			case DriverMSSQL:
				db = &MSSQL{}
				table = "dbo.orders"
				queryTable = table
			case DriverSqlite:
				db = &SQLite{}
				connection.URL = "file:../testdata/sqlite/lazysql.sqlite3?mode=ro"
			default:
				t.Fatalf("unexpected provider %s", connection.Provider)
			}
			if err := db.Connect(ctx, connection.URL); err != nil {
				t.Fatalf("connect: %v", err)
			}
			switch d := db.(type) {
			case *MySQL:
				pool = d.Connection
			case *Postgres:
				pool = d.Connection
			case *MSSQL:
				pool = d.Connection
			case *SQLite:
				pool = d.Connection
			}
			defer pool.Close()
			page, err := db.GetRecords(ctx, connection.DBName, table, "", "id", 0, 2)
			if err != nil {
				t.Fatal(err)
			}
			if len(page.Rows) != 3 || !page.HasNextPage {
				t.Fatalf("unexpected page %+v", page)
			}
			count, err := db.GetExactRowCount(ctx, connection.DBName, table, "")
			if err != nil || count != 3000 {
				t.Fatalf("exact count = %d, %v", count, err)
			}
			if _, err = db.GetEstimatedRowCount(ctx, connection.DBName, table); err != nil {
				t.Fatal(err)
			}
			columns, err := db.GetTableColumns(ctx, connection.DBName, table)
			if err != nil {
				t.Fatal(err)
			}
			bulk, err := db.(BulkTableColumnLoader).GetTableColumnsBulk(ctx, connection.DBName, []string{table})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(columns, bulk[table]) {
				t.Fatalf("single columns %v differ from bulk %v", columns, bulk[table])
			}
			for name, load := range map[string]func(context.Context, string, string) ([][]string, error){"constraints": db.GetConstraints, "foreign_keys": db.GetForeignKeys, "reverse_foreign_keys": db.GetReferencingTables, "indexes": db.GetIndexes} {
				rows, err := load(ctx, connection.DBName, table)
				if err != nil || len(rows) < 2 {
					t.Errorf("%s: rows %v, error %v", name, rows, err)
				}
			}
			emitted := 0
			result, err := db.(QueryStreamer).StreamQuery(ctx, "SELECT id FROM "+queryTable+" ORDER BY id", 2, func(batch QueryBatch) error { emitted += len(batch.Rows); return nil })
			if err != nil || !result.Truncated || result.Rows != 2 || emitted != 2 {
				t.Fatalf("stream: %+v, emitted %d, error %v", result, emitted, err)
			}
			if pool.Stats().InUse != 0 {
				t.Fatal("connection remained in use")
			}
			if postgres, ok := db.(*Postgres); ok {
				testPostgresPendingChangeRouting(t, ctx, postgres)
			}

		})
	}
}

// Both catalogs deliberately contain the same schema/table/primary key. A
// pending edit must use its captured database, never the login's database.
func testPostgresPendingChangeRouting(t *testing.T, ctx context.Context, db *Postgres) {
	t.Helper()
	target, _, err := db.connectionFor(ctx, "postgres")
	if err != nil {
		t.Fatal(err)
	}
	defer target.Close()
	schema := fmt.Sprintf("pr346_routing_%d", time.Now().UnixNano())
	table := schema + ".route"
	for _, conn := range []*sql.DB{db.Connection, target} {
		if _, err := conn.ExecContext(ctx, "CREATE SCHEMA "+schema); err != nil {
			t.Fatal(err)
		}
		defer func(conn *sql.DB) {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if _, err := conn.ExecContext(cleanupCtx, "DROP SCHEMA "+schema+" CASCADE"); err != nil {
				t.Errorf("cleanup routing schema: %v", err)
			}
		}(conn)
		if _, err := conn.ExecContext(ctx, "CREATE TABLE "+table+" (id integer PRIMARY KEY, name text); INSERT INTO "+table+" VALUES (1, 'original')"); err != nil {
			t.Fatal(err)
		}
	}
	err = db.ExecutePendingChanges(ctx, []models.DBDMLChange{{
		Database: "postgres", Table: table, Type: models.DMLUpdateType,
		Values:         []models.CellValue{{Column: "name", Value: "changed", Type: models.String}},
		PrimaryKeyInfo: []models.PrimaryKeyInfo{{Name: "id", Value: 1}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for conn, want := range map[*sql.DB]string{db.Connection: "original", target: "changed"} {
		var got string
		if err := conn.QueryRowContext(ctx, "SELECT name FROM "+table+" WHERE id = 1").Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("pending edit routed to wrong database: got %q, want %q", got, want)
		}
	}
}
