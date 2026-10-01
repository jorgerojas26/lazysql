package drivers

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestSQLiteRowCountsSeparateEstimateFromExact(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	driver := &SQLite{Connection: db}
	estimate, err := driver.GetEstimatedRowCount(context.Background(), "main", "orders")
	if err != nil {
		t.Fatalf("GetEstimatedRowCount() error = %v", err)
	}
	if estimate != nil {
		t.Fatalf("SQLite estimate = %v, want unavailable", *estimate)
	}

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM `orders`").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(12))
	count, err := driver.GetExactRowCount(context.Background(), "main", "orders", "")
	if err != nil || count != 12 {
		t.Fatalf("GetExactRowCount() = %d, %v; want 12", count, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLRowCountsUseContextAndNativeEstimate(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	driver := &MySQL{Connection: db}
	mock.ExpectQuery("SELECT TABLE_ROWS").WithArgs("shop", "orders").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_ROWS"}).AddRow(42))
	estimate, err := driver.GetEstimatedRowCount(context.Background(), "shop", "orders")
	if err != nil || estimate == nil || *estimate != 42 {
		t.Fatalf("GetEstimatedRowCount() = %v, %v; want 42", estimate, err)
	}

	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM `shop`\\.`orders` WHERE status = 1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(7))
	count, err := driver.GetExactRowCount(context.Background(), "shop", "orders", "WHERE status = 1")
	if err != nil || count != 7 {
		t.Fatalf("GetExactRowCount() = %d, %v; want 7", count, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPostgresRowCountsUseSchemaAndContext(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	driver := &Postgres{Connection: db, CurrentDatabase: "shop"}
	mock.ExpectQuery("SELECT c\\.reltuples::bigint").WithArgs("public", "orders").
		WillReturnRows(sqlmock.NewRows([]string{"reltuples"}).AddRow(123))
	estimate, err := driver.GetEstimatedRowCount(context.Background(), "shop", "public.orders")
	if err != nil || estimate == nil || *estimate != 123 {
		t.Fatalf("GetEstimatedRowCount() = %v, %v; want 123", estimate, err)
	}

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM "public"\."orders" WHERE id > 10`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(11))
	count, err := driver.GetExactRowCount(context.Background(), "shop", "public.orders", "WHERE id > 10")
	if err != nil || count != 11 {
		t.Fatalf("GetExactRowCount() = %d, %v; want 11", count, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMSSQLRowCountsUseContext(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	driver := &MSSQL{Connection: db, isAzureSQL: true}
	mock.ExpectQuery("SELECT SUM\\(row_count\\)").WithArgs("orders").
		WillReturnRows(sqlmock.NewRows([]string{"rows"}).AddRow(88))
	estimate, err := driver.GetEstimatedRowCount(context.Background(), "shop", "orders")
	if err != nil || estimate == nil || *estimate != 88 {
		t.Fatalf("GetEstimatedRowCount() = %v, %v; want 88", estimate, err)
	}

	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM \[orders\] WHERE active = 1`).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(6))
	count, err := driver.GetExactRowCount(context.Background(), "shop", "orders", "WHERE active = 1")
	if err != nil || count != 6 {
		t.Fatalf("GetExactRowCount() = %d, %v; want 6", count, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestExactRowCountHonorsCanceledContext(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = (&SQLite{Connection: db}).GetExactRowCount(ctx, "main", "orders", "")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("GetExactRowCount() error = %v, want context.Canceled", err)
	}
}
