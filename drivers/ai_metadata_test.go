package drivers

import (
	"context"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMySQLOutgoingForeignKeysForAI(t *testing.T) {
	connection, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	mock.ExpectQuery("SELECT CONSTRAINT_NAME AS constraint_name").WithArgs("shop", "orders", "shop").WillReturnRows(sqlmock.NewRows([]string{"constraint_name", "column_name", "referenced_table", "referenced_column"}).AddRow("customer_fk", "tenant_id", "customers", "tenant_id").AddRow("customer_fk", "customer_id", "customers", "id"))
	rows, err := (&MySQL{Connection: connection}).GetOutgoingForeignKeys(context.Background(), "shop", "orders")
	if err != nil || len(rows) != 3 || rows[2][2] != "customers" {
		t.Fatalf("outgoing keys = %v %v", rows, err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMySQLOutgoingForeignKeysHonorCancellation(t *testing.T) {
	connection, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = (&MySQL{Connection: connection}).GetOutgoingForeignKeys(ctx, "shop", "orders")
	if err == nil || !strings.Contains(err.Error(), "cancel") {
		t.Fatalf("canceled metadata request = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}
