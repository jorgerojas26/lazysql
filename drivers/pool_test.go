package drivers

import (
	"context"
	"database/sql"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"github.com/jorgerojas26/lazysql/models"
)

func TestApplyConnectionPoolConfig(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	err = applyConnectionPoolConfig(db, models.ConnectionPoolConfig{
		MaxOpenConnections: 4,
		MaxIdleConnections: 3,
	})
	if err != nil {
		t.Fatalf("applyConnectionPoolConfig() error = %v", err)
	}

	stats := db.Stats()
	if stats.MaxOpenConnections != 4 {
		t.Errorf("MaxOpenConnections = %d, want 4", stats.MaxOpenConnections)
	}
	connections := make([]*sql.Conn, 0, 4)
	for range 4 {
		connection, err := db.Conn(context.Background())
		if err != nil {
			t.Fatalf("db.Conn() error = %v", err)
		}
		connections = append(connections, connection)
	}
	if got := db.Stats().OpenConnections; got != 4 {
		t.Fatalf("OpenConnections = %d, want 4", got)
	}
	for _, connection := range connections {
		if err := connection.Close(); err != nil {
			t.Fatalf("connection.Close() error = %v", err)
		}
	}
	if got := db.Stats().Idle; got != 3 {
		t.Errorf("Idle = %d, want 3 after applying max idle limit", got)
	}
}

func TestApplyConnectionPoolConfigUsesDefaultsForZero(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	if err := applyConnectionPoolConfig(db, models.ConnectionPoolConfig{}); err != nil {
		t.Fatalf("applyConnectionPoolConfig() error = %v", err)
	}

	if got := db.Stats().MaxOpenConnections; got != models.DefaultMaxOpenConnections {
		t.Errorf("MaxOpenConnections = %d, want %d", got, models.DefaultMaxOpenConnections)
	}

	connections := make([]*sql.Conn, 0, models.DefaultMaxIdleConnections)
	for range models.DefaultMaxIdleConnections {
		connection, err := db.Conn(context.Background())
		if err != nil {
			t.Fatalf("db.Conn() error = %v", err)
		}
		connections = append(connections, connection)
	}
	for _, connection := range connections {
		if err := connection.Close(); err != nil {
			t.Fatalf("connection.Close() error = %v", err)
		}
	}
	if got := db.Stats().Idle; got != models.DefaultMaxIdleConnections {
		t.Errorf("Idle = %d, want %d", got, models.DefaultMaxIdleConnections)
	}
}

func TestApplySQLitePoolConfigKeepsInMemoryDatabaseOnOneConnection(t *testing.T) {
	db := &SQLite{}
	if err := db.Connect(":memory:"); err != nil {
		t.Fatalf("SQLite.Connect() error = %v", err)
	}
	defer db.Connection.Close()

	stats := db.Connection.Stats()
	if stats.MaxOpenConnections != 1 {
		t.Errorf("MaxOpenConnections = %d, want 1", stats.MaxOpenConnections)
	}
	if stats.Idle != 1 {
		t.Errorf("Idle = %d, want 1", stats.Idle)
	}

	if _, err := db.Connection.Exec("CREATE TABLE test (value TEXT)"); err != nil {
		t.Fatalf("create table error = %v", err)
	}
	if _, err := db.Connection.Exec("INSERT INTO test (value) VALUES ('ok')"); err != nil {
		t.Fatalf("insert error = %v", err)
	}

	var value string
	if err := db.Connection.QueryRow("SELECT value FROM test").Scan(&value); err != nil {
		t.Fatalf("query error = %v", err)
	}
	if value != "ok" {
		t.Errorf("value = %q, want %q", value, "ok")
	}
}
