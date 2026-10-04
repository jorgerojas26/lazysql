package components

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/jorgerojas26/lazysql/drivers"
)

func TestBinaryCellsMySQLIntegration(t *testing.T) {
	url := os.Getenv("LAZYSQL_TEST_MYSQL_URL")
	if url == "" {
		t.Skip("disposable MySQL URL not set")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	db := &drivers.MySQL{}
	if err := db.Connect(ctx, url); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Connection.Close(); err != nil {
			t.Error(err)
		}
	})
	var database string
	if err := db.Connection.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&database); err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("lazysql_binary_%d", time.Now().UnixNano())
	if _, err := db.Connection.ExecContext(ctx, "CREATE TABLE "+name+" (id BINARY(16) PRIMARY KEY, token VARBINARY(32) NULL, note TEXT)"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := db.Connection.ExecContext(context.Background(), "DROP TABLE "+name); err != nil {
			t.Error(err)
		}
	})
	if _, err := db.Connection.ExecContext(ctx, "INSERT INTO "+name+" VALUES (?, ?, ?), (?, ?, ?)", binaryID, "\x00\x01\xff", "first", "abcdefghijklmnop", "AB", "second"); err != nil {
		t.Fatal(err)
	}
	table := newBinaryEditingTable(t)
	table.DBDriver = db
	table.state.databaseName = database
	table.state.tableName = name
	load := func() {
		t.Helper()
		page, err := db.GetRecords(ctx, database, name, "", "note", 0, 100)
		if err != nil {
			t.Fatal(err)
		}
		columns, err := db.GetTableColumns(ctx, database, name)
		if err != nil {
			t.Fatal(err)
		}
		table.state.columns = nil
		table.state.records = page.Rows
		table.state.rawCellValues = nil
		table.Clear()
		table.AddRows(page.Rows)
		table.SetColumns(columns)
		*table.state.listOfDBChanges = nil
	}
	apply := func() {
		t.Helper()
		if err := db.ExecutePendingChanges(ctx, *table.state.listOfDBChanges); err != nil {
			t.Fatal(err)
		}
	}
	check := func(id, token string) {
		t.Helper()
		var got string
		if err := db.Connection.QueryRowContext(ctx, "SELECT HEX(token) FROM "+name+" WHERE id = ?", id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != token {
			t.Fatalf("stored token = %q, want %q", got, token)
		}
	}
	load()
	if got := cellText(table.GetCell(2, 0)); got != "0x6162636465666768696A6B6C6D6E6F70" {
		t.Fatalf("printable BINARY(16) display = %q", got)
	}
	editBinaryCell(t, table, 1, 1, "0xDEADBEEF")
	apply()
	check(binaryID, "DEADBEEF")
	load()
	editBinaryCell(t, table, 2, 1, "0xCAFE")
	apply()
	check("abcdefghijklmnop", "CAFE")
	load()
	table.Select(1, 0)
	table.duplicateRow()
	editBinaryCell(t, table, 2, 0, "0x0123456789ABCDEF0123456789ABCDEF")
	apply()
	check("\x01\x23\x45\x67\x89\xab\xcd\xef\x01\x23\x45\x67\x89\xab\xcd\xef", "DEADBEEF")
}
