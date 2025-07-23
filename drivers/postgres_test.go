package drivers

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/lib/pq"

	"github.com/jorgerojas26/lazysql/models"
)

const (
	DBNamePostgres    = "postgres"
	schemaPostgres    = "public"
	tableNamePostgres = "test_table"
)

var schemaAndTablePostgres = fmt.Sprintf("%s.%s", schemaPostgres, tableNamePostgres)

func TestPostgres_FormatArg_SpecialCharacters(t *testing.T) {
	db := &Postgres{}

	testCases := []struct {
		name     string
		arg      any
		expected string
	}{
		{
			name:     "String with single quote",
			arg:      "O'Reilly",
			expected: "'O''Reilly'",
		},
		{
			name:     "String with backslash",
			arg:      "C:\\Program Files",
			expected: "'C:\\Program Files'",
		},
		{
			name:     "String with double quotes",
			arg:      `"quoted"`,
			expected: `'"quoted"'`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			formattedArg := db.FormatArgForQueryString(tc.arg)
			if formattedArg != tc.expected {
				t.Fatalf("expected %q, but got %q", tc.expected, formattedArg)
			}
		})
	}
}

func TestPostgres_FormatArg(t *testing.T) {
	db := &Postgres{}

	testCases := []struct {
		name     string
		arg      any
		expected string
	}{
		{
			name:     "Integer argument",
			arg:      123,
			expected: "123",
		},
		{
			name:     "String argument",
			arg:      "test string",
			expected: "'test string'",
		},
		{
			name:     "Byte array argument",
			arg:      []byte("byte array"),
			expected: "[98 121 116 101 32 97 114 114 97 121]",
		},
		{
			name:     "Float argument",
			arg:      123.45,
			expected: "123.45",
		},
		{
			name:     "Boolean true",
			arg:      true,
			expected: "true",
		},
		{
			name:     "Boolean false",
			arg:      false,
			expected: "false",
		},
		{
			name:     "NULL value",
			arg:      nil,
			expected: "<nil>",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			formattedArg := db.FormatArgForQueryString(tc.arg)
			if formattedArg != tc.expected {
				t.Fatalf("expected %q, but got %q", tc.expected, formattedArg)
			}
		})
	}
}

func TestPostgres_DMLChangeToQueryString(t *testing.T) {
	db := &Postgres{}

	testCases := []struct {
		name     string
		change   models.DBDMLChange
		expected string
	}{
		{
			name: "Insert with int value",
			change: models.DBDMLChange{
				Table: schemaAndTablePostgres,
				Type:  models.DMLInsertType,
				Values: []models.CellValue{
					{Column: "name", Value: "test_name", Type: models.String},
					{Column: "value", Value: 123, Type: models.String},
				},
			},
			// expected: `INSERT INTO "test_db"."test_table" (name, value) VALUES ('test_name', 123)`,
			expected: fmt.Sprintf(`INSERT INTO "%s"."%s" (name, value) VALUES ('test_name', 123)`, schemaPostgres, tableNamePostgres),
		},
		{
			name: "Update with string value",
			change: models.DBDMLChange{
				Table: schemaAndTablePostgres,
				Type:  models.DMLUpdateType,
				Values: []models.CellValue{
					{Column: "name", Value: "test_name", Type: models.String},
					{Column: "value", Value: "123", Type: models.String},
				},
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{Name: "id", Value: "1"},
				},
			},
			expected: fmt.Sprintf(`UPDATE "%s"."%s" SET "name" = 'test_name', "value" = '123' WHERE "id" = '1'`, schemaPostgres, tableNamePostgres),
		},
		{
			name: "Delete with UUID value",
			change: models.DBDMLChange{
				Table: schemaAndTablePostgres,
				Type:  models.DMLDeleteType,
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{Name: "id", Value: "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"},
				},
			},
			expected: fmt.Sprintf(`DELETE FROM "%s"."%s" WHERE "id" = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'`, schemaPostgres, tableNamePostgres),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			queryString, err := db.DMLChangeToQueryString(tc.change)
			if err != nil {
				t.Fatalf("DMLChangeToQueryString failed: %v", err)
			}
			if queryString != tc.expected {
				t.Fatalf("Expected:\n%q\nGot:\n%q", tc.expected, queryString)
			}
		})
	}
}

func TestPostgres_Connect_Mock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db}
	mock.ExpectPing()

	err = pg.Connection.Ping()
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_ErrorScenarios(t *testing.T) {
	testCases := []struct {
		name      string
		setupMock func(mock sqlmock.Sqlmock)
		testFunc  func(db *Postgres) error
	}{
		{
			name: "GetTables error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT table_name, table_schema FROM information_schema.tables WHERE table_catalog = \\$1").WithArgs(schemaPostgres).
					WillReturnError(errors.New("query error"))
			},
			testFunc: func(db *Postgres) error {
				_, err := db.GetTables(schemaPostgres)
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("Error creating mock: %v", err)
			}
			defer db.Close()

			tc.setupMock(mock)
			pg := &Postgres{Connection: db, CurrentDatabase: schemaPostgres}

			err = tc.testFunc(pg)
			if err == nil {
				t.Error("Expected error but got nil")
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("Unfulfilled expectations: %s", err)
			}
		})
	}
}

func TestPostgres_GetTableColumns_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db, CurrentDatabase: DBNamePostgres}
	mock.ExpectQuery("SELECT column_name, data_type, is_nullable, column_default FROM information_schema.columns WHERE table_catalog = \\$1 AND table_schema = \\$2 AND table_name = \\$3 ORDER by ordinal_position").WithArgs(DBNamePostgres, schemaPostgres, tableNamePostgres).
		WillReturnError(errors.New("query error"))

	_, err = pg.GetTableColumns(DBNamePostgres, schemaAndTablePostgres)
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_GetRecords(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db, CurrentDatabase: DBNamePostgres}

	columns := []string{"id", "name"}
	rows := sqlmock.NewRows(columns).
		AddRow(1, "Alice").
		AddRow(2, "Bob")

	mock.ExpectQuery(fmt.Sprintf(`SELECT \* FROM "%s"."%s" LIMIT \$1 OFFSET \$2`, schemaPostgres, tableNamePostgres)).WithArgs(DefaultRowLimit, 0).WillReturnRows(rows)

	mock.ExpectQuery(fmt.Sprintf(`SELECT COUNT\(\*\) FROM "%s"."%s"`, schemaPostgres, tableNamePostgres)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	records, total, err := pg.GetRecords(DBNamePostgres, schemaAndTablePostgres, "", "", 0, DefaultRowLimit)
	if err != nil {
		t.Fatalf("GetRecords failed: %v", err)
	}

	expected := [][]string{
		{"id", "name"},
		{"1", "Alice"},
		{"2", "Bob"},
	}

	if !reflect.DeepEqual(records, expected) {
		t.Fatalf("Expected %v, got %v", expected, records)
	}

	if total != 2 {
		t.Fatalf("Expected total 2, got %d", total)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_GetForeignKeys(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db, CurrentDatabase: DBNamePostgres}

	rows := sqlmock.NewRows([]string{
		"constraint_name", "column_name",
		"foreign_table_name", "foreign_column_name",
		"update_rule", "delete_rule",
	}).AddRow(
		"fk_test", "user_id",
		"users", "id",
		"CASCADE", "SET NULL",
	)

	mock.ExpectQuery(fmt.Sprintf(`
        SELECT
            tc.constraint_name,
            kcu.column_name,
            ccu.table_name AS foreign_table_name,
            ccu.column_name AS foreign_column_name
        FROM
            information_schema.table_constraints AS tc
            JOIN information_schema.key_column_usage AS kcu ON tc.constraint_name = kcu.constraint_name
            AND tc.table_schema = kcu.table_schema
            JOIN information_schema.constraint_column_usage AS ccu ON ccu.constraint_name = tc.constraint_name
            AND ccu.table_schema = tc.table_schema
        WHERE
            tc.constraint_type = 'FOREIGN KEY'
          	AND tc.table_schema = '%s'
            AND tc.table_name = '%s'
  `, schemaPostgres, tableNamePostgres)).WillReturnRows(rows)

	constraints, err := pg.GetForeignKeys(DBNamePostgres, schemaAndTablePostgres)
	if err != nil {
		t.Fatalf("GetForeignKeys failed: %v", err)
	}

	expected := [][]string{
		{"constraint_name", "column_name", "foreign_table_name", "foreign_column_name", "update_rule", "delete_rule"},
		{"fk_test", "user_id", "users", "id", "CASCADE", "SET NULL"},
	}

	if !reflect.DeepEqual(constraints, expected) {
		t.Fatalf("Expected %v, got %v", expected, constraints)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_GetIndexes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db, CurrentDatabase: DBNamePostgres}

	rows := sqlmock.NewRows([]string{"indexname", "indexdef"}).
		AddRow("idx_name", "CREATE INDEX idx_name ON test_table USING btree (name)")

	mock.ExpectQuery(fmt.Sprintf(`
        SELECT
            i.relname AS index_name,
            a.attname AS column_name,
            am.amname AS type
        FROM
            pg_namespace n,
            pg_class t,
            pg_class i,
            pg_index ix,
            pg_attribute a,
            pg_am am
        WHERE
            t.oid = ix.indrelid
            and i.oid = ix.indexrelid
            and a.attrelid = t.oid
            and a.attnum = ANY\(ix.indkey\)
            and t.relkind = 'r'
            and am.oid = i.relam
          	and n.oid = t.relnamespace
            and n.nspname = '%s'
            and t.relname = '%s'
        ORDER BY
            t.relname,
            i.relname
  `, schemaPostgres, tableNamePostgres)).WillReturnRows(rows)

	indexes, err := pg.GetIndexes(DBNamePostgres, schemaAndTablePostgres)
	if err != nil {
		t.Fatalf("GetIndexes failed: %v", err)
	}

	expected := [][]string{
		{"indexname", "indexdef"},
		{"idx_name", "CREATE INDEX idx_name ON test_table USING btree (name)"},
	}

	if !reflect.DeepEqual(indexes, expected) {
		t.Fatalf("Expected %v, got %v", expected, indexes)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_Transactions(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectRollback()
	mock.ExpectBegin()
	mock.ExpectCommit()

	// Test Begin and Rollback
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}

	// Test Begin and Commit
	tx, err = db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_ExecutePendingChanges(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db, CurrentDatabase: DBNamePostgres}

	changes := []models.DBDMLChange{
		{
			Table: schemaAndTablePostgres,
			Type:  models.DMLUpdateType,
			Values: []models.CellValue{
				{Column: "name", Value: "New Name", Type: models.String},
			},
			PrimaryKeyInfo: []models.PrimaryKeyInfo{
				{Name: "id", Value: 1},
			},
		},
	}

	mock.ExpectBegin()
	mock.ExpectExec(fmt.Sprintf(`UPDATE "%s"."%s" SET "name" = \$1 WHERE "id" = \$2`, schemaPostgres, tableNamePostgres)).WithArgs("New Name", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = pg.ExecutePendingChanges(changes)
	if err != nil {
		t.Fatalf("ExecutePendingChanges failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_GetPrimaryKeyColumnNames(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &Postgres{Connection: db, CurrentDatabase: DBNamePostgres}

	rows := sqlmock.NewRows([]string{"column_name"}).
		AddRow("id")

	mock.ExpectQuery(`
		SELECT
			a.attname AS column_name
		FROM
			pg_index i
			JOIN pg_class c ON c.oid = i.indrelid
			JOIN pg_attribute a ON a.attrelid = c.oid
				AND a.attnum = ANY \(i.indkey\)
			JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE
			relname = \$2 AND nspname = \$1 AND indisprimary
	`).WithArgs(schemaPostgres, tableNamePostgres).WillReturnRows(rows)

	keys, err := pg.GetPrimaryKeyColumnNames(DBNamePostgres, schemaAndTablePostgres)
	if err != nil {
		t.Fatalf("GetPrimaryKeyColumnNames failed: %v", err)
	}

	expected := []string{"id"}
	if !reflect.DeepEqual(keys, expected) {
		t.Fatalf("Expected %v, got %v", expected, keys)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestPostgres_SetGetProvider(t *testing.T) {
	db := &Postgres{}
	db.SetProvider(DriverPostgres)

	if db.GetProvider() != DriverPostgres {
		t.Fatalf("Provider mismatch: got %s, expected %s", db.GetProvider(), DriverPostgres)
	}
}

func TestPostgres_formatTableName(t *testing.T) {
	db := &Postgres{}

	splitTableString := strings.Split(schemaAndTablePostgres, ".")

	tableSchema := splitTableString[0]
	name := splitTableString[1]

	tableName, err := db.formatTableName(schemaAndTablePostgres)
	if err != nil {
		t.Fatalf("formatTableName failed: %v", err)
	}

	expected := fmt.Sprintf("\"%s\".\"%s\"", tableSchema, name)

	if tableName != expected {
		t.Fatalf("formatTableName failed: got %s, expected %s", tableName, expected)
	}
}
