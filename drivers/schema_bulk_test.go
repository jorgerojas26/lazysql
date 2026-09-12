package drivers

import "testing"

func TestSQLiteGetTableColumnsBulkReturnsAllRequestedTables(t *testing.T) {
	db := &SQLite{}
	if err := db.Connect(":memory:"); err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Connection.Close() })

	if _, err := db.Connection.Exec(`CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT); CREATE TABLE orders (id INTEGER, user_id INTEGER);`); err != nil {
		t.Fatalf("create tables error = %v", err)
	}

	columns, err := db.GetTableColumnsBulk("main", []string{"users", "orders"})
	if err != nil {
		t.Fatalf("GetTableColumnsBulk() error = %v", err)
	}
	if got := columns["users"]; len(got) != 3 || got[1][0] != "id" || got[2][0] != "name" {
		t.Fatalf("users columns = %#v", got)
	}
	if got := columns["orders"]; len(got) != 3 || got[1][0] != "id" || got[2][0] != "user_id" {
		t.Fatalf("orders columns = %#v", got)
	}
}
