package drivers

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	_ "modernc.org/sqlite"

	"github.com/jorgerojas26/lazysql/models"
)

const (
	testDBNameSQLite      = "test_db"
	testDBTableNameSQLite = "test_table"
)

func TestSQLite_FormatArg_SpecialCharacters(t *testing.T) {
	db := &SQLite{}

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

func TestSQLite_FormatArgForQueryString(t *testing.T) {
	db := &SQLite{}

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
			name:     "Default argument",
			arg:      nil,
			expected: "NULL",
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

func TestSQLite_DMLChangeToQueryString(t *testing.T) {
	db := &SQLite{}

	testCases := []struct {
		name     string
		change   models.DBDMLChange
		expected string
	}{
		{
			name: "Insert with int value",
			change: models.DBDMLChange{
				Database: testDBNameSQLite, Table: testDBTableNameSQLite,
				Type: models.DMLInsertType,
				Values: []models.CellValue{
					{Column: "name", Value: "test_name", Type: models.String},
					{Column: "value", Value: 123, Type: models.String},
				},
			},
			expected: fmt.Sprintf("INSERT INTO %s (name, value) VALUES ('test_name', 123)", db.formatTableName(testDBTableNameSQLite)),
		},
		{
			name: "Update with string value",
			change: models.DBDMLChange{
				Database: testDBNameSQLite, Table: testDBTableNameSQLite,
				Type: models.DMLUpdateType,
				Values: []models.CellValue{
					{Column: "name", Value: "test_name", Type: models.String},
					{Column: "value", Value: "123", Type: models.String},
				},
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{Name: "id", Value: "1"},
				},
			},
			expected: fmt.Sprintf("UPDATE %s SET `name` = 'test_name', `value` = '123' WHERE `id` = '1'", db.formatTableName(testDBTableNameSQLite)),
		},
		{
			name: "Delete with int value",
			change: models.DBDMLChange{
				Database: testDBNameSQLite, Table: testDBTableNameSQLite,
				Type: models.DMLDeleteType,
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{Name: "id", Value: 1},
				},
			},
			expected: fmt.Sprintf("DELETE FROM %s WHERE `id` = 1", db.formatTableName(testDBTableNameSQLite)),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			queryString, err := db.DMLChangeToQueryString(tc.change)
			if err != nil {
				t.Fatalf("DMLChangeToQueryString failed: %v", err)
			}
			if queryString != tc.expected {
				t.Fatalf("Expected: %q\nGot: %q", tc.expected, queryString)
			}
		})
	}
}

func TestSQLite_Connect_Mock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	sqlite := &SQLite{Connection: db}
	mock.ExpectPing()

	err = sqlite.Connection.Ping()
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestSQLite_ErrorScenarios(t *testing.T) {
	testCases := []struct {
		name      string
		setupMock func(mock sqlmock.Sqlmock)
		testFunc  func(db *SQLite) error
	}{
		{
			name: "GetTables error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT name FROM sqlite_master WHERE type='table'").
					WillReturnError(errors.New("query error"))
			},
			testFunc: func(db *SQLite) error {
				_, err := db.GetTables(testDBNameSQLite)
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
			sqlite := &SQLite{Connection: db}

			err = tc.testFunc(sqlite)
			if err == nil {
				t.Error("Expected error but got nil")
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("Unfulfilled expectations: %s", err)
			}
		})
	}
}

func TestSQLite_GetTableColumns_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	sqlite := &SQLite{Connection: db}
	mock.ExpectQuery(fmt.Sprintf("PRAGMA table_info\\(%s\\)", sqlite.formatTableName(testDBTableNameSQLite))).
		WillReturnError(errors.New("query error"))

	_, err = sqlite.GetTableColumns(testDBNameSQLite, testDBTableNameSQLite)
	if err == nil {
		t.Fatal("Expected error but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestSQLite_GetRecords(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}

	defer db.Close()

	sqlite := &SQLite{Connection: db}

	columns := []string{"id", "name"}
	rows := sqlmock.NewRows(columns).
		AddRow(1, "Alice").
		AddRow(2, "Bob")

	mock.ExpectQuery(fmt.Sprintf("SELECT \\* FROM %s LIMIT \\?, \\?", sqlite.formatTableName(testDBTableNameSQLite))).
		WithArgs(0, DefaultRowLimit).
		WillReturnRows(rows)

	mock.ExpectQuery(fmt.Sprintf("SELECT COUNT\\(\\*\\) FROM %s", sqlite.formatTableName(testDBTableNameSQLite))).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	records, total, _, err := sqlite.GetRecords(testDBNameSQLite, testDBTableNameSQLite, "", "", 0, DefaultRowLimit)
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

func TestSQLite_GetForeignKeys(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	sqlite := &SQLite{Connection: db}

	rows := sqlmock.NewRows([]string{"id", "seq", "table", "from", "to", "on_update", "on_delete", "match"}).
		AddRow(0, 0, "users", "user_id", "id", "CASCADE", "SET NULL", "NONE")

	mock.ExpectQuery(fmt.Sprintf("PRAGMA foreign_key_list\\(%s\\)", sqlite.formatTableName(testDBTableNameSQLite))).
		WillReturnRows(rows)

	constraints, err := sqlite.GetForeignKeys(testDBNameSQLite, testDBTableNameSQLite)
	if err != nil {
		t.Fatalf("GetForeignKeys failed: %v", err)
	}

	expected := [][]string{
		{"id", "seq", "table", "from", "to", "on_update", "on_delete", "match"},
		{"0", "0", "users", "user_id", "id", "CASCADE", "SET NULL", "NONE"},
	}

	if !reflect.DeepEqual(constraints, expected) {
		t.Fatalf("Expected %v, got %v", expected, constraints)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestSQLite_GetIndexes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	sqlite := &SQLite{Connection: db}

	// Mock index list
	mock.ExpectQuery(fmt.Sprintf("PRAGMA index_list\\(%s\\)", sqlite.formatTableName(testDBTableNameSQLite))).
		WillReturnRows(sqlmock.NewRows([]string{"seq", "name", "unique", "origin", "partial", "columns"}).
			AddRow(0, "idx_name", 1, "", "", "name"))

	// // Mock index info
	// mock.ExpectQuery("PRAGMA index_info\\(idx_name\\)").
	// 	WillReturnRows(sqlmock.NewRows([]string{"seqno", "cid", "name"}).
	// 		AddRow(0, 1, "name"))

	indexes, err := sqlite.GetIndexes(testDBNameSQLite, testDBTableNameSQLite)
	if err != nil {
		t.Fatalf("GetIndexes failed: %v", err)
	}

	expected := [][]string{
		{"seq", "name", "unique", "origin", "partial", "columns"},
		{"0", "idx_name", "1", "", "", "name"},
	}

	if !reflect.DeepEqual(indexes, expected) {
		t.Fatalf("Expected %v, got %v", expected, indexes)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestSQLite_Transactions(t *testing.T) {
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

func TestSQLite_ExecutePendingChanges(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	sqlite := &SQLite{Connection: db}

	changes := []models.DBDMLChange{
		{
			Database: testDBNameSQLite,
			Table:    testDBTableNameSQLite,
			Type:     models.DMLUpdateType,
			Values: []models.CellValue{
				{Column: "name", Value: "New Name", Type: models.String},
			},
			PrimaryKeyInfo: []models.PrimaryKeyInfo{
				{Name: "id", Value: 1},
			},
		},
	}

	mock.ExpectBegin()
	mock.ExpectExec(fmt.Sprintf("UPDATE %s SET `name` = \\? WHERE `id` = \\?", sqlite.formatTableName(testDBTableNameSQLite))).
		WithArgs("New Name", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = sqlite.ExecutePendingChanges(changes)
	if err != nil {
		t.Fatalf("ExecutePendingChanges failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestSQLite_GetPrimaryKeyColumnNames(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	sqlite := &SQLite{Connection: db}

	rows := sqlmock.NewRows([]string{"cid", "name", "type", "notnull", "dflt_value", "pk"}).
		AddRow(0, "id", "INTEGER", 1, nil, 1).
		AddRow(1, "name", "TEXT", 0, nil, 0)

	mock.ExpectQuery(fmt.Sprintf("PRAGMA table_info\\(%s\\)", sqlite.formatTableName(testDBTableNameSQLite))).
		WillReturnRows(rows)

	keys, err := sqlite.GetPrimaryKeyColumnNames(testDBNameSQLite, testDBTableNameSQLite)
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

func TestSQLite_SetProvider(t *testing.T) {
	db := &SQLite{}
	db.SetProvider(DriverSqlite)

	if db.Provider != DriverSqlite {
		t.Fatalf("SetProvider failed: got %q, expected %q", db.Provider, DriverSqlite)
	}
}

func TestSQLite_GetProvider(t *testing.T) {
	db := &SQLite{Provider: DriverSqlite}

	provider := db.GetProvider()
	if provider != DriverSqlite {
		t.Fatalf("GetProvider failed: got %q, expected %q", provider, DriverSqlite)
	}
}

func TestSQLite_formatTableName(t *testing.T) {
	db := &SQLite{}

	tableName := db.formatTableName(testDBTableNameSQLite)
	expectedTableName := fmt.Sprintf("`%s`", testDBTableNameSQLite)

	if tableName != expectedTableName {
		t.Fatalf("formatTableName failed: got %q, expected %q", tableName, expectedTableName)
	}
}
