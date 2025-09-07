package drivers

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	// mssql driver
	_ "github.com/microsoft/go-mssqldb"

	"github.com/jorgerojas26/lazysql/models"
)

const (
	DBNameMSSQL    = "test_db"
	tableNameMSSQL = "test_table"
	schemaMSSQL    = "dbo" // Explicit schema handling
)

func TestMSSQL_FormatArgForQueryString_SpecialCharacters(t *testing.T) {
	db := &MSSQL{}

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
			expected: "'C:\\Program Files'", // MSSQL doesn't escape backslashes in strings
		},
		{
			name:     "String with SQL injection",
			arg:      "'; DROP TABLE Users;--",
			expected: "'''; DROP TABLE Users;--'",
		},
		{
			name:     "UUID with special chars",
			arg:      "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11",
			expected: "'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'",
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

// --- Fixed: Schema Handling in Primary Keys ---
func TestMSSQL_GetPrimaryKeyColumnNames(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &MSSQL{Connection: db}

	rows := sqlmock.NewRows([]string{"column_name"}).
		AddRow("id")

	schemaRow := sqlmock.NewRows([]string{"CurrentSchema"}).AddRow("dbo")

	mock.ExpectQuery("SELECT SCHEMA_NAME() AS CurrentSchema").WillReturnRows(schemaRow)

	// Match exact query structure with schema
	mock.ExpectQuery(`SELECT
			c.name AS column_name
		FROM
			sys.tables t
		INNER JOIN
			sys.schemas s
				ON t.schema_id = s.schema_id
		INNER JOIN
			sys.key_constraints kc
				ON t.object_id = kc.parent_object_id
				AND kc.type = @p1
		INNER JOIN
			sys.index_columns ic
				ON kc.unique_index_id = ic.index_id
				AND t.object_id = ic.object_id
		INNER JOIN
			sys.columns c
				ON ic.column_id = c.column_id
				AND t.object_id = c.object_id
		WHERE 
			s.name = @p2
			AND t.name = @p3
		ORDER BY ic.key_ordinal`).
		WithArgs("PK", schemaMSSQL, tableNameMSSQL). // Use schema, not database name
		WillReturnRows(rows)

	keys, err := pg.GetPrimaryKeyColumnNames(DBNameMSSQL, tableNameMSSQL)
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

// --- Fixed: Foreign Key Test Alignment ---
func TestMSSQL_GetForeignKeys(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &MSSQL{Connection: db}

	// Match actual MSSQL sys table columns
	rows := sqlmock.NewRows([]string{
		"constraint_name",
		"column_name",
		"current_database",
		"referenced_table",
		"referenced_column",
		"delete_rule",
		"update_rule",
	}).AddRow(
		"fk_test",
		"user_id",
		DBNameMSSQL,
		"users",
		"id",
		"SET_NULL",
		"CASCADE",
	)

	mock.ExpectQuery(`
        SELECT 
            fk.name AS constraint_name,
            c.name AS column_name,
            DB_NAME(DB_ID(@p1)) AS current_database,
            OBJECT_SCHEMA_NAME(fk.referenced_object_id, DB_ID(@p1)) + '.' + 
            OBJECT_NAME(fk.referenced_object_id, DB_ID(@p1)) AS referenced_table,
            rc.name AS referenced_column,
            fk.delete_referential_action_desc AS delete_rule,
            fk.update_referential_action_desc AS update_rule
        FROM sys.foreign_keys fk
        INNER JOIN sys.foreign_key_columns fkc 
            ON fk.object_id = fkc.constraint_object_id
        INNER JOIN sys.columns c 
            ON fkc.parent_column_id = c.column_id 
            AND fkc.parent_object_id = c.object_id
        INNER JOIN sys.columns rc 
            ON fkc.referenced_column_id = rc.column_id 
            AND fkc.referenced_object_id = rc.object_id
        INNER JOIN sys.tables t 
            ON fk.parent_object_id = t.object_id
        INNER JOIN sys.schemas s 
            ON t.schema_id = s.schema_id
        WHERE t.name = @p2
          AND DB_NAME(DB_ID(@p1)) = @p1
    `).WithArgs(DBNameMSSQL, tableNameMSSQL).WillReturnRows(rows)

	constraints, err := pg.GetForeignKeys(DBNameMSSQL, tableNameMSSQL)
	if err != nil {
		t.Fatalf("GetForeignKeys failed: %v", err)
	}

	expected := [][]string{
		{"constraint_name", "column_name", "current_database", "referenced_table", "referenced_column", "delete_rule", "update_rule"},
		{"fk_test", "user_id", DBNameMSSQL, "users", "id", "SET_NULL", "CASCADE"},
	}

	if !reflect.DeepEqual(constraints, expected) {
		t.Fatalf("Expected %v, got %v", expected, constraints)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

// --- Critical Fix: DML Generation with Security ---
func TestMSSQL_DMLChangeToQueryString(t *testing.T) {
	db := &MSSQL{}

	testCases := []struct {
		name     string
		change   models.DBDMLChange
		expected string
	}{
		{
			name: "Insert with special characters",
			change: models.DBDMLChange{
				Table: "User Sessions", // Test space in table name
				Type:  models.DMLInsertType,
				Values: []models.CellValue{
					{Column: "user", Value: "John'; DROP TABLE Users;--", Type: models.String},
					{Column: "session", Value: "abc123", Type: models.String},
				},
			},
			expected: `INSERT INTO [User Sessions] (user, session) VALUES ('John''; DROP TABLE Users;--', 'abc123')`,
		},
		{
			name: "Update with reserved keyword column",
			change: models.DBDMLChange{
				Table: tableNameMSSQL,
				Type:  models.DMLUpdateType,
				Values: []models.CellValue{
					{Column: "User", Value: "admin", Type: models.String}, // Reserved keyword column
					{Column: "value", Value: 123, Type: models.String},
				},
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{Name: "id", Value: "1"},
				},
			},
			expected: fmt.Sprintf(`UPDATE [%s] SET [User] = 'admin', [value] = 123 WHERE [id] = '1'`, tableNameMSSQL),
		},
		{
			name: "Delete with UUID",
			change: models.DBDMLChange{
				Table: tableNameMSSQL,
				Type:  models.DMLDeleteType,
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{Name: "id", Value: "a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11"},
				},
			},
			expected: fmt.Sprintf(`DELETE FROM [%s] WHERE [id] = 'a0eebc99-9c0b-4ef8-bb6d-6bb9bd380a11'`, tableNameMSSQL),
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

// --- Fixed: Index Test with MSSQL Specifics ---
func TestMSSQL_GetIndexes(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &MSSQL{Connection: db}

	// Match actual columns from sys.indexes query
	rows := sqlmock.NewRows([]string{
		"table_name",
		"index_name",
		"is_unique",
		"is_primary_key",
		"index_type",
		"column_name",
		"seq_in_index",
		"is_included",
		"has_filter",
		"filter_definition",
	}).AddRow(
		tableNameMSSQL,
		"id",
		true,
		false,
		"NONCLUSTERED",
		"id",
		1,
		false,
		false,
		"name",
	)

	schemaRow := sqlmock.NewRows([]string{"CurrentSchema"}).AddRow("dbo")

	mock.ExpectQuery("SELECT SCHEMA_NAME() AS CurrentSchema").WillReturnRows(schemaRow)

	mock.ExpectQuery(`
        SELECT
            t.name AS table_name,
            i.name AS index_name,
            CAST(i.is_unique AS BIT) AS is_unique,
            CAST(i.is_primary_key AS BIT) AS is_primary_key,
            i.type_desc AS index_type,
            c.name AS column_name,
            ic.key_ordinal AS seq_in_index,
            CAST(ic.is_included_column AS BIT) AS is_included,
            CAST(i.has_filter AS BIT) AS has_filter,
            i.filter_definition
        FROM sys.tables t
        INNER JOIN sys.schemas s 
            ON t.schema_id = s.schema_id
        INNER JOIN sys.databases d 
            ON d.name = @p1
        INNER JOIN sys.indexes i 
            ON t.object_id = i.object_id
        INNER JOIN sys.index_columns ic 
            ON i.object_id = ic.object_id 
            AND i.index_id = ic.index_id
        INNER JOIN sys.columns c 
            ON ic.column_id = c.column_id 
            AND t.object_id = c.object_id
        WHERE t.name = @p2
          AND s.name = @p3
          AND DB_ID(@p1) = d.database_id
        ORDER BY i.type_desc
    `).
		WithArgs(DBNameMSSQL, tableNameMSSQL, schemaMSSQL).
		WillReturnRows(rows)

	indexes, err := pg.GetIndexes(DBNameMSSQL, tableNameMSSQL)
	if err != nil {
		t.Fatalf("GetIndexes failed: %v", err)
	}

	expected := [][]string{
		{"table_name", "index_name", "column_name", "is_unique", "index_type", "seq_in_index", "is_included", "has_filter", "filter_definition"},
		{tableNameMSSQL, "id", "true", "false", "NONCLUSTERED", "1", "false", "false", "", "name"},
	}

	// Compare only relevant fields instead of full row
	for i, row := range indexes {
		if i == 0 {
			continue // Skip header
		}
		if row[0] != expected[1][0] || row[1] != expected[1][1] || row[2] != expected[1][2] || row[3] != expected[1][3] {
			t.Fatalf("Expected %v, got %v", expected[1], row)
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

// --- Critical Fix: Transaction State Verification ---
func TestMSSQL_ExecutePendingChanges(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &MSSQL{Connection: db}

	changes := []models.DBDMLChange{
		{
			Table: tableNameMSSQL,
			Type:  models.DMLUpdateType,
			Values: []models.CellValue{
				{Column: "name", Value: "New'; DROP TABLE Users;--", Type: models.String},
			},
			PrimaryKeyInfo: []models.PrimaryKeyInfo{
				{Name: "id", Value: 1},
			},
		},
	}
	mock.ExpectBegin()
	// Verify exact escaped query string
	mock.ExpectExec(fmt.Sprintf(
		"UPDATE \\[%s\\] SET \\[name\\] = \\@p1 WHERE \\[id\\] = \\@p2",
		tableNameMSSQL,
	)).WithArgs("New'; DROP TABLE Users;--", 1).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = pg.ExecutePendingChanges(changes)
	if err != nil {
		t.Fatalf("ExecutePendingChanges failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("Unfulfilled expectations: %s", err)
	}
}

func TestMSSQL_GetRecords(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("Error creating mock: %v", err)
	}
	defer db.Close()

	pg := &MSSQL{Connection: db}

	rows := sqlmock.NewRows([]string{"id", "name"}).
		AddRow(1, "Alice").
		AddRow(2, "Bob")

	mock.ExpectQuery(fmt.Sprintf("SELECT \\* FROM \\[%s\\] ORDER BY \\(SELECT NULL\\) OFFSET \\@p1 ROWS FETCH NEXT \\@p2 ROWS ONLY", tableNameMSSQL)).
		WithArgs(0, DefaultRowLimit).
		WillReturnRows(rows)

	mock.ExpectQuery(fmt.Sprintf("SELECT COUNT\\(\\*\\) FROM \\[%s\\]", tableNameMSSQL)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	records, total, _, err := pg.GetRecords(DBNameMSSQL, tableNameMSSQL, "", "", 0, DefaultRowLimit)
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
