package drivers

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/go-sql-driver/mysql"

	"github.com/jorgerojas26/lazysql/models"
)

const (
	testDBNameMySQL      = "test_db"
	testDBTableNameMySQL = "test_table"
)

func TestMySQL_FormatArg_SpecialCharacters(t *testing.T) {
	db := &MySQL{}

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

func TestMySQL_FormatArgForQueryString(t *testing.T) {
	db := &MySQL{}

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
			expected: "'byte array'",
		},
		{
			name:     "Float argument",
			arg:      123.45,
			expected: "123.45",
		},
		{
			name:     "Integer-looking float",
			arg:      5.0,
			expected: "5.0",
		},
		{
			name:     "Simple decimal",
			arg:      2.5,
			expected: "2.5",
		},
		{
			name:     "Float with trailing zeros",
			arg:      3.0000,
			expected: "3.0",
		},
		{
			name:     "Float with multiple decimal places",
			arg:      123.456789,
			expected: "123.456789",
		},
		{
			name:     "Float with zero value",
			arg:      0.0,
			expected: "0.0",
		},
		{
			name:     "Float with mixed decimal places",
			arg:      98.6000,
			expected: "98.6",
		},
		{
			name:     "Float32 value",
			arg:      float32(3.5),
			expected: "3.5",
		},
		{
			name:     "Float with small decimal",
			arg:      0.00001,
			expected: "0.00001",
		},
		{
			name:     "Float with no decimal part",
			arg:      100.0,
			expected: "100.0",
		},
		{
			name:     "Float with exact precision",
			arg:      2.50000,
			expected: "2.5",
		},
		{
			name:     "Default argument",
			arg:      true,
			expected: "true",
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

func TestMySQL_DMLChangeToQueryString(t *testing.T) {
	db := &MySQL{}

	testCases := []struct {
		name     string
		change   models.DBDMLChange
		expected string
	}{
		{
			name: "Insert with int value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLInsertType,
				Values: []models.CellValue{
					{
						Column: "name",
						Value:  "test_name",
						Type:   models.String,
					},
					{
						Column: "value",
						Value:  123,
						Type:   models.String,
					},
				},
			},
			expected: fmt.Sprintf("INSERT INTO %s (name, value) VALUES ('test_name', 123)", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Insert with  string value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLInsertType,
				Values: []models.CellValue{
					{
						Column: "name",
						Value:  "test_name",
						Type:   models.String,
					},
					{
						Column: "value",
						Value:  "123",
						Type:   models.String,
					},
				},
			},
			expected: fmt.Sprintf("INSERT INTO %s (name, value) VALUES ('test_name', '123')", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Insert with  byte array value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLInsertType,
				Values: []models.CellValue{
					{
						Column: "name",
						Value:  "test_name",
						Type:   models.String,
					},
					{
						Column: "value",
						Value:  []byte("123"),
						Type:   models.String,
					},
				},
			},
			expected: fmt.Sprintf("INSERT INTO %s (name, value) VALUES ('test_name', '123')", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Insert with  float value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLInsertType,
				Values: []models.CellValue{
					{
						Column: "name",
						Value:  "test_name",
						Type:   models.String,
					},
					{
						Column: "value",
						Value:  123.45,
						Type:   models.String,
					},
				},
			},
			expected: fmt.Sprintf("INSERT INTO %s (name, value) VALUES ('test_name', 123.45)", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Insert with  bool value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLInsertType,
				Values: []models.CellValue{
					{
						Column: "name",
						Value:  "test_name",
						Type:   models.String,
					},
					{
						Column: "value",
						Value:  true,
						Type:   models.String,
					},
				},
			},
			expected: fmt.Sprintf("INSERT INTO %s (name, value) VALUES ('test_name', true)", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Insert with  default value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLInsertType,
				Values: []models.CellValue{
					{
						Column: "name",
						Value:  "test_name",
						Type:   models.String,
					},
					{
						Column: "value",
						Value:  nil,
						Type:   models.Default,
					},
				},
			},
			expected: fmt.Sprintf("INSERT INTO %s (name, value) VALUES ('test_name', DEFAULT)", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Insert with empty value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLInsertType,
				Values: []models.CellValue{
					{
						Column: "name",
						Value:  "test_name",
						Type:   models.String,
					},
					{
						Column: "value",
						Value:  "",
						Type:   models.Empty,
					},
				},
			},
			expected: fmt.Sprintf("INSERT INTO %s (name, value) VALUES ('test_name', '')", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Insert with NULL value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLInsertType,
				Values: []models.CellValue{
					{
						Column: "name",
						Value:  "test_name",
						Type:   models.String,
					},
					{
						Column: "value",
						Value:  nil,
						Type:   models.Null,
					},
				},
			},
			expected: fmt.Sprintf("INSERT INTO %s (name, value) VALUES ('test_name', NULL)", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Update with int value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLUpdateType,
				Values: []models.CellValue{
					{
						Column: "name",
						Value:  "test_name",
						Type:   models.String,
					},
					{
						Column: "value",
						Value:  123,
						Type:   models.String,
					},
				},
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{
						Name:  "id",
						Value: 1,
					},
				},
			},
			expected: fmt.Sprintf("UPDATE %s SET `name` = 'test_name', `value` = 123 WHERE `id` = 1", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Update with string value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLUpdateType,
				Values: []models.CellValue{
					{
						Column: "name",
						Value:  "test_name",
						Type:   models.String,
					},
					{
						Column: "value",
						Value:  "123",
						Type:   models.String,
					},
				},
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{
						Name:  "id",
						Value: "1",
					},
				},
			},
			expected: fmt.Sprintf("UPDATE %s SET `name` = 'test_name', `value` = '123' WHERE `id` = '1'", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Update with byte array value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLUpdateType,
				Values: []models.CellValue{
					{
						Column: "name",
						Value:  "test_name",
						Type:   models.String,
					},
					{
						Column: "value",
						Value:  []byte("123"),
						Type:   models.String,
					},
				},
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{
						Name:  "id",
						Value: "1",
					},
				},
			},
			expected: fmt.Sprintf("UPDATE %s SET `name` = 'test_name', `value` = '123' WHERE `id` = '1'", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Update with float value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLUpdateType,
				Values: []models.CellValue{
					{
						Column: "name",
						Value:  "test_name",
						Type:   models.String,
					},
					{
						Column: "value",
						Value:  123.45,
						Type:   models.String,
					},
				},
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{
						Name:  "id",
						Value: "1",
					},
				},
			},
			expected: fmt.Sprintf("UPDATE %s SET `name` = 'test_name', `value` = 123.45 WHERE `id` = '1'", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Update with bool value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLUpdateType,
				Values: []models.CellValue{
					{
						Column: "name",
						Value:  "test_name",
						Type:   models.String,
					},
					{
						Column: "value",
						Value:  true,
						Type:   models.String,
					},
				},
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{
						Name:  "id",
						Value: "1",
					},
				},
			},
			expected: fmt.Sprintf("UPDATE %s SET `name` = 'test_name', `value` = true WHERE `id` = '1'", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Update with default value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLUpdateType,
				Values: []models.CellValue{
					{
						Column: "name",
						Value:  "test_name",
						Type:   models.String,
					},
					{
						Column: "value",
						Value:  nil,
						Type:   models.Default,
					},
				},
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{
						Name:  "id",
						Value: "1",
					},
				},
			},
			expected: fmt.Sprintf("UPDATE %s SET `name` = 'test_name', `value` = DEFAULT WHERE `id` = '1'", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Update with empty value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLUpdateType,
				Values: []models.CellValue{
					{
						Column: "name",
						Value:  "test_name",
						Type:   models.String,
					},
					{
						Column: "value",
						Value:  "",
						Type:   models.Empty,
					},
				},
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{
						Name:  "id",
						Value: "1",
					},
				},
			},
			expected: fmt.Sprintf("UPDATE %s SET `name` = 'test_name', `value` = '' WHERE `id` = '1'", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Update with NULL value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLUpdateType,
				Values: []models.CellValue{
					{
						Column: "name",
						Value:  "test_name",
						Type:   models.String,
					},
					{
						Column: "value",
						Value:  nil,
						Type:   models.Null,
					},
				},
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{
						Name:  "id",
						Value: "1",
					},
				},
			},
			expected: fmt.Sprintf("UPDATE %s SET `name` = 'test_name', `value` = NULL WHERE `id` = '1'", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Delete with int value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLDeleteType,
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{
						Name:  "id",
						Value: 1,
					},
				},
			},
			expected: fmt.Sprintf("DELETE FROM %s WHERE `id` = 1", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Delete with string value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLDeleteType,
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{
						Name:  "id",
						Value: "1",
					},
				},
			},
			expected: fmt.Sprintf("DELETE FROM %s WHERE `id` = '1'", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Delete with byte array value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLDeleteType,
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{
						Name:  "id",
						Value: []byte("1"),
					},
				},
			},
			expected: fmt.Sprintf("DELETE FROM %s WHERE `id` = '1'", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Delete with float value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLDeleteType,
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{
						Name:  "id",
						Value: 1.0,
					},
				},
			},
			expected: fmt.Sprintf("DELETE FROM %s WHERE `id` = 1.0", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
		{
			name: "Delete with bool value",
			change: models.DBDMLChange{
				Database: testDBNameMySQL, Table: testDBTableNameMySQL,
				Type: models.DMLDeleteType,
				PrimaryKeyInfo: []models.PrimaryKeyInfo{
					{
						Name:  "id",
						Value: true,
					},
				},
			},
			expected: fmt.Sprintf("DELETE FROM %s WHERE `id` = true", db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			queryString, err := db.DMLChangeToQueryString(tc.change)
			if err != nil {
				t.Fatalf("DMLChangeToQueryString failed: %v", err)
			}
			if queryString != tc.expected {
				t.Fatalf("DMLChangeToQueryString returned unexpected query string: got %q, expected %q", queryString, tc.expected)
			}
		})
	}
}

func TestMySQL_Connect_Mock(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectPing()

	err = mysql.Connection.Ping()
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_ErrorScenarios(t *testing.T) {
	testCases := []struct {
		name      string
		setupMock func(mock sqlmock.Sqlmock)
		testFunc  func(db *MySQL) error
	}{
		{
			name: "GetDatabases error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SHOW DATABASES").WillReturnError(errors.New("query error"))
			},
			testFunc: func(db *MySQL) error {
				_, err := db.GetDatabases()
				return err
			},
		},
		{
			name: "GetTables error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(fmt.Sprintf("SHOW TABLES FROM `%s`", testDBNameMySQL)).WillReturnError(errors.New("query error"))
			},
			testFunc: func(db *MySQL) error {
				_, err := db.GetTables("test_db")
				return err
			},
		},
		{
			name: "Empty database name",
			setupMock: func(_ sqlmock.Sqlmock) {
				// No expectations needed for this case
			},
			testFunc: func(db *MySQL) error {
				_, err := db.GetTables("")
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
			}
			defer db.Close()

			tc.setupMock(mock)

			mysql := &MySQL{Connection: db}

			err = tc.testFunc(mysql)
			if err == nil {
				t.Error("Expected error, but got nil")
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unfulfilled expectations: %s", err)
			}
		})
	}
}

func TestMySQL_GetTableColumns_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectQuery(fmt.Sprintf("DESCRIBE %s", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).WillReturnError(errors.New("query error"))

	_, err = mysql.GetTableColumns(testDBNameMySQL, testDBTableNameMySQL)

	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetConstraints_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}

	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectQuery("SELECT CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA = \\? AND TABLE_NAME = \\?").WithArgs(testDBNameMySQL, testDBTableNameMySQL).WillReturnError(errors.New("query error"))

	_, err = mysql.GetConstraints(testDBNameMySQL, testDBTableNameMySQL)

	log.Println("errrorrrrrr", err.Error())

	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetForeignKeys_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectQuery("SELECT TABLE_NAME, COLUMN_NAME, CONSTRAINT_NAME, REFERENCED_COLUMN_NAME, REFERENCED_TABLE_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE REFERENCED_TABLE_SCHEMA = \\? AND REFERENCED_TABLE_NAME = \\?").WithArgs(testDBNameMySQL, testDBTableNameMySQL).WillReturnError(errors.New("query error"))

	_, err = mysql.GetForeignKeys(testDBNameMySQL, testDBTableNameMySQL)

	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetIndexes_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectQuery(fmt.Sprintf("SHOW INDEX FROM %s", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).WillReturnError(errors.New("query error"))

	_, err = mysql.GetIndexes(testDBNameMySQL, testDBTableNameMySQL)

	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetRecords_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	testCases := []struct {
		name      string
		setupMock func(mock sqlmock.Sqlmock)
		testFunc  func(db *MySQL) error
	}{
		{
			name: "GetRecords error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(fmt.Sprintf("SELECT \\* FROM %s LIMIT \\?, \\?", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).WithArgs(0, DefaultRowLimit).WillReturnError(errors.New("query error"))
			},
			testFunc: func(db *MySQL) error {
				_, _, err := db.GetRecords("test_db", "test_table", "", "", 0, DefaultRowLimit)
				return err
			},
		},
		{
			name: "GetRecords with where error",
			setupMock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(fmt.Sprintf("SELECT \\* FROM %s WHERE id = 1 LIMIT \\?, \\?", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).WithArgs(0, DefaultRowLimit).WillReturnError(errors.New("query error"))
			},
			testFunc: func(db *MySQL) error {
				_, _, err := db.GetRecords("test_db", "test_table", "WHERE id = 1", "", 0, DefaultRowLimit)
				return err
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			tc.setupMock(mock)
			err = tc.testFunc(mysql)
			if err == nil {
				t.Error("Expected error, but got nil")
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unfulfilled expectations: %s", err)
			}
		})
	}
}

func TestMySQL_ExecuteQuery_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectQuery(fmt.Sprintf("SELECT \\* FROM %s", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).WillReturnError(errors.New("query error"))

	_, _, err = mysql.ExecuteQuery(fmt.Sprintf("SELECT * FROM %s", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL)))

	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_UpdateRecord_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectExec(fmt.Sprintf("UPDATE %s SET name = \\? WHERE id = \\?", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).WithArgs("updated_test", "1").WillReturnError(errors.New("query error"))

	err = mysql.UpdateRecord(testDBNameMySQL, testDBTableNameMySQL, "name", "updated_test", "id", "1")

	log.Println(err.Error())

	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_DeleteRecord_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectExec(fmt.Sprintf("DELETE FROM %s WHERE id = ?", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).WithArgs("1").WillReturnError(errors.New("query error"))

	err = mysql.DeleteRecord(testDBNameMySQL, testDBTableNameMySQL, "id", "1")

	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_ExecuteDMLStatement_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectExec("UPDATE test_table SET value = 3 WHERE name = 'test1'").WillReturnError(errors.New("query error"))

	_, err = mysql.ExecuteDMLStatement(fmt.Sprintf("UPDATE %s SET value = 3 WHERE name = 'test1'", testDBTableNameMySQL))

	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_ExecutePendingChanges_PartialFailure(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	changes := []models.DBDMLChange{
		{
			Database: "test_db",
			Table:    "test_table",
			Type:     models.DMLUpdateType,
			Values: []models.CellValue{
				{
					Column: "value",
					Value:  "3",
					Type:   models.String,
				},
			},
			PrimaryKeyInfo: []models.PrimaryKeyInfo{
				{
					Name:  "id",
					Value: "1",
				},
			},
		},
		{
			Database: "test_db",
			Table:    "test_table",
			Type:     models.DMLUpdateType,
			Values: []models.CellValue{
				{
					Column: "value",
					Value:  "4",
					Type:   models.String,
				},
			},
			PrimaryKeyInfo: []models.PrimaryKeyInfo{
				{
					Name:  "id",
					Value: "2",
				},
			},
		},
	}

	mock.ExpectBegin()
	mock.ExpectExec(fmt.Sprintf("UPDATE %s SET `value` = \\? WHERE `id` = \\?", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).WithArgs("3", "1").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec(fmt.Sprintf("UPDATE %s SET `value` = \\? WHERE `id` = \\?", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).WithArgs("4", "2").WillReturnError(errors.New("query error"))
	mock.ExpectRollback()

	err = mysql.ExecutePendingChanges(changes)

	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_ExecutePendingChanges_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	changes := []models.DBDMLChange{
		{
			Database: "test_db",
			Table:    "test_table",
			Type:     models.DMLUpdateType,
			Values: []models.CellValue{
				{
					Column: "value",
					Value:  "3",
					Type:   models.String,
				},
			},
			PrimaryKeyInfo: []models.PrimaryKeyInfo{
				{
					Name:  "id",
					Value: "1",
				},
			},
		},
	}

	mock.ExpectBegin()
	mock.ExpectExec(fmt.Sprintf("UPDATE %s SET `value` = \\? WHERE `id` = \\?", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).WithArgs("3", "1").WillReturnError(errors.New("query error"))
	mock.ExpectRollback()

	err = mysql.ExecutePendingChanges(changes)

	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetPrimaryKeyColumnNames_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectQuery("SELECT column_name FROM information_schema.key_column_usage WHERE table_schema = \\? AND table_name = \\? AND constraint_name = \\?").WithArgs(testDBNameMySQL, testDBTableNameMySQL, "PRIMARY").WillReturnError(errors.New("query error"))

	_, err = mysql.GetPrimaryKeyColumnNames(testDBNameMySQL, testDBTableNameMySQL)

	log.Println(err.Error())

	if err == nil {
		t.Fatalf("Expected error, but got nil")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_TestConnection(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock database: %v", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectPing()

	err = mysql.Connection.Ping()
	if err != nil {
		t.Fatalf("Connection test failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("there were unfulfilled expectations: %s", err)
	}
}

func TestMySQL_Transactions(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected", err)
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
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

// func TestMySQL_ConnectionPooling(t *testing.T) {
// 	db, mock, err := sqlmock.New()
// 	if err != nil {
// 		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
// 	}
// 	defer db.Close()
//
// 	mock.ExpectQuery("PRAGMA index_list\\(test_table\\)").WillReturnError(errors.New("query error"))
//
// 	mysql := &MySQL{Connection: db}
//
// 	// Test multiple connections
// 	err = mysql.Connect(testDBNameMySQL)
// 	if err != nil {
// 		t.Fatalf("Failed to connect: %v", err)
// 	}
// 	defer cleanupMySQLTestDB(t, mysql)
//
// 	// Verify connection is reusable
// 	for range 3 {
// 		_, err := mysql.GetDatabases()
// 		if err != nil {
// 			t.Fatalf("Failed to use connection: %v", err)
// 		}
// 	}
// }

// func TestMySQL_Connect(t *testing.T) {
// 	db, mock, err := sqlmock.New()
// 	if err != nil {
// 		t.Fatalf("an error '%s' was not expected when opening a stub database connection", err)
// 	}
// 	defer db.Close()
//
// 	mock.ExpectQuery("PRAGMA index_list\\(test_table\\)").WillReturnError(errors.New("query error"))
//
// 	mysql := &MySQL{Connection: db}
//
// 	err = mysql.Connect(testDBNameMySQL)
// 	if err != nil {
// 		t.Fatalf("Connect failed: %v", err)
// 	}
//
// 	defer cleanupMySQLTestDB(t, mysql)
// }

func TestMySQL_GetDatabases(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("an error '%s' was not expected", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	rows := sqlmock.NewRows([]string{"Database"}).
		AddRow(testDBNameMySQL)

	mock.ExpectQuery("SHOW DATABASES").WillReturnRows(rows)

	databases, err := mysql.GetDatabases()
	if err != nil {
		t.Fatalf("GetDatabases failed: %v", err)
	}

	expected := []string{"test_db"}
	if !reflect.DeepEqual(databases, expected) {
		t.Fatalf("Expected %v, got %v", expected, databases)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	// Set up mock expectations
	rows := sqlmock.NewRows([]string{"Tables_in_test_db"}).
		AddRow("test_table").
		AddRow("another_table")

	mock.ExpectQuery("SHOW TABLES FROM `test_db`").WillReturnRows(rows)

	tables, err := mysql.GetTables("test_db")
	if err != nil {
		t.Fatalf("GetTables failed: %v", err)
	}

	expected := map[string][]string{
		"test_db": {"test_table", "another_table"},
	}

	if !reflect.DeepEqual(tables, expected) {
		t.Fatalf("Expected %v, got %v", expected, tables)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetTableColumns(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	// Set up mock expectations
	rows := sqlmock.NewRows([]string{"Field", "Type", "Null", "Key", "Default", "Extra"}).
		AddRow("id", "int(11)", "NO", "PRI", nil, "auto_increment").
		AddRow("name", "varchar(255)", "YES", "", nil, "")

	mock.ExpectQuery(fmt.Sprintf("DESCRIBE %s", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).WillReturnRows(rows)

	columns, err := mysql.GetTableColumns(testDBNameMySQL, testDBTableNameMySQL)
	if err != nil {
		t.Fatalf("GetTableColumns failed: %v", err)
	}

	expected := [][]string{
		{"Field", "Type", "Null", "Key", "Default", "Extra"},
		{"id", "int(11)", "NO", "PRI", "", "auto_increment"},
		{"name", "varchar(255)", "YES", "", "", ""},
	}

	if !reflect.DeepEqual(columns, expected) {
		t.Fatalf("Expected %v, got %v", expected, columns)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetConstraints(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	// Set up mock expectations
	rows := sqlmock.NewRows([]string{"CONSTRAINT_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME"}).
		AddRow("fk_test", "user_id", "users", "id")

	mock.ExpectQuery("SELECT CONSTRAINT_NAME, COLUMN_NAME, REFERENCED_TABLE_NAME, REFERENCED_COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA = \\? AND TABLE_NAME = \\?").
		WithArgs(testDBNameMySQL, testDBTableNameMySQL).
		WillReturnRows(rows)

	constraints, err := mysql.GetConstraints(testDBNameMySQL, testDBTableNameMySQL)
	if err != nil {
		t.Fatalf("GetConstraints failed: %v", err)
	}

	expected := [][]string{
		{"CONSTRAINT_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME"},
		{"fk_test", "user_id", "users", "id"},
	}

	if !reflect.DeepEqual(constraints, expected) {
		t.Fatalf("Expected %v, got %v", expected, constraints)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetForeignKeys(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	// Set up mock expectations
	rows := sqlmock.NewRows([]string{
		"TABLE_NAME",
		"COLUMN_NAME",
		"CONSTRAINT_NAME",
		"REFERENCED_COLUMN_NAME",
		"REFERENCED_TABLE_NAME",
	}).AddRow("orders", "user_id", "fk_user", "id", "users")

	mock.ExpectQuery(
		"SELECT TABLE_NAME, COLUMN_NAME, CONSTRAINT_NAME, REFERENCED_COLUMN_NAME, REFERENCED_TABLE_NAME "+
			"FROM information_schema.KEY_COLUMN_USAGE "+
			"WHERE REFERENCED_TABLE_SCHEMA = \\? AND REFERENCED_TABLE_NAME = \\?").
		WithArgs(testDBNameMySQL, testDBTableNameMySQL).
		WillReturnRows(rows)

	foreignKeys, err := mysql.GetForeignKeys(testDBNameMySQL, testDBTableNameMySQL)
	if err != nil {
		t.Fatalf("GetForeignKeys failed: %v", err)
	}

	expected := [][]string{
		{"TABLE_NAME", "COLUMN_NAME", "CONSTRAINT_NAME", "REFERENCED_COLUMN_NAME", "REFERENCED_TABLE_NAME"},
		{"orders", "user_id", "fk_user", "id", "users"},
	}

	if !reflect.DeepEqual(foreignKeys, expected) {
		t.Fatalf("Expected:\n%v\nGot:\n%v", expected, foreignKeys)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetIndexes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	// Set up mock expectations
	rows := sqlmock.NewRows([]string{
		"Seq", "Key_name", "Non_unique", "Index_type", "Column_name",
	}).AddRow(1, "idx_name", 0, "BTREE", "name").
		AddRow(2, "PRIMARY", 0, "BTREE", "id")

	mock.ExpectQuery(fmt.Sprintf("SHOW INDEX FROM %s", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).
		WillReturnRows(rows)

	indexes, err := mysql.GetIndexes(testDBNameMySQL, testDBTableNameMySQL)
	if err != nil {
		t.Fatalf("GetIndexes failed: %v", err)
	}

	expected := [][]string{
		{"Seq", "Key_name", "Non_unique", "Index_type", "Column_name"},
		{"1", "idx_name", "0", "BTREE", "name"},
		{"2", "PRIMARY", "0", "BTREE", "id"},
	}

	if !reflect.DeepEqual(indexes, expected) {
		t.Fatalf("Expected:\n%v\nGot:\n%v", expected, indexes)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetRecords(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	// Set up mock expectations
	columns := []string{"id", "name", "value"}
	rows := sqlmock.NewRows(columns).
		AddRow(1, "test1", 100).
		AddRow(2, "test2", 200)

	mock.ExpectQuery(fmt.Sprintf("SELECT \\* FROM %s LIMIT \\?, \\?", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).
		WithArgs(0, DefaultRowLimit).
		WillReturnRows(rows)

	mock.ExpectQuery(fmt.Sprintf("SELECT COUNT\\(\\*\\) FROM %s", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(2))

	records, total, err := mysql.GetRecords(testDBNameMySQL, testDBTableNameMySQL, "", "", 0, DefaultRowLimit)
	if err != nil {
		t.Fatalf("GetRecords failed: %v", err)
	}

	if total != 2 {
		t.Fatalf("Expected total 2, got %d", total)
	}

	expectedRecords := [][]string{
		{"id", "name", "value"},
		{"1", "test1", "100"},
		{"2", "test2", "200"},
	}

	if !reflect.DeepEqual(records, expectedRecords) {
		t.Fatalf("Expected %v, got %v", expectedRecords, records)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_ExecuteQuery(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	// Set up mock expectations
	columns := []string{"id", "name"}
	rows := sqlmock.NewRows(columns).
		AddRow(1, "Alice").
		AddRow(2, "Bob")

	mock.ExpectQuery(fmt.Sprintf("SELECT \\* FROM %s", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).
		WillReturnRows(rows)

	results, _, err := mysql.ExecuteQuery(fmt.Sprintf("SELECT * FROM %s", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL)))
	if err != nil {
		t.Fatalf("ExecuteQuery failed: %v", err)
	}

	expectedResults := [][]string{
		{"id", "name"},
		{"1", "Alice"},
		{"2", "Bob"},
	}

	if !reflect.DeepEqual(results, expectedResults) {
		t.Fatalf("Expected results:\n%v\nGot:\n%v", expectedResults, results)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_UpdateRecord(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectExec(fmt.Sprintf("UPDATE %s SET name = \\? WHERE id = \\?", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).
		WithArgs("new_name", "1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = mysql.UpdateRecord(testDBNameMySQL, testDBTableNameMySQL, "name", "new_name", "id", "1")
	if err != nil {
		t.Fatalf("UpdateRecord failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_DeleteRecord(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	mock.ExpectExec(fmt.Sprintf("DELETE FROM %s WHERE id = \\?", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).
		WithArgs("1").
		WillReturnResult(sqlmock.NewResult(0, 1))

	err = mysql.DeleteRecord(testDBNameMySQL, testDBTableNameMySQL, "id", "1")
	if err != nil {
		t.Fatalf("DeleteRecord failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_ExecuteDMLStatement(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	// Set up mock expectations
	mock.ExpectExec(fmt.Sprintf("UPDATE %s SET value = 3 WHERE name = 'test1'", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).
		WillReturnResult(sqlmock.NewResult(0, 2))

	result, err := mysql.ExecuteDMLStatement(fmt.Sprintf("UPDATE %s SET value = 3 WHERE name = 'test1'", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL)))
	if err != nil {
		t.Fatalf("ExecuteDMLStatement failed: %v", err)
	}

	expected := "2 rows affected"
	if result != expected {
		t.Fatalf("Expected %q, got %q", expected, result)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_ExecutePendingChanges(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	changes := []models.DBDMLChange{
		{
			Database: testDBNameMySQL,
			Table:    testDBTableNameMySQL,
			Type:     models.DMLUpdateType,
			Values: []models.CellValue{
				{Column: "name", Value: "New Name", Type: models.String},
			},
			PrimaryKeyInfo: []models.PrimaryKeyInfo{
				{Name: "id", Value: 1},
			},
		},
		{
			Database: testDBNameMySQL,
			Table:    testDBTableNameMySQL,
			Type:     models.DMLDeleteType,
			PrimaryKeyInfo: []models.PrimaryKeyInfo{
				{Name: "id", Value: 2},
			},
		},
	}

	// Set up transaction expectations
	mock.ExpectBegin()
	mock.ExpectExec(fmt.Sprintf("UPDATE %s SET `name` = \\? WHERE `id` = \\?", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).
		WithArgs("New Name", 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	mock.ExpectExec(fmt.Sprintf("DELETE FROM %s WHERE `id` = \\?", mysql.formatTableName(testDBNameMySQL, testDBTableNameMySQL))).
		WithArgs(2).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err = mysql.ExecutePendingChanges(changes)
	if err != nil {
		t.Fatalf("ExecutePendingChanges failed: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_GetPrimaryKeyColumnNames(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("error creating mock: %s", err)
	}
	defer db.Close()

	mysql := &MySQL{Connection: db}

	rows := sqlmock.NewRows([]string{"column_name"}).
		AddRow("id").
		AddRow("uuid")

	mock.ExpectQuery("SELECT column_name FROM information_schema.key_column_usage WHERE table_schema = \\? AND table_name = \\? AND constraint_name = \\?").
		WithArgs(testDBNameMySQL, testDBTableNameMySQL, "PRIMARY").
		WillReturnRows(rows)

	keys, err := mysql.GetPrimaryKeyColumnNames(testDBNameMySQL, testDBTableNameMySQL)
	if err != nil {
		t.Fatalf("GetPrimaryKeyColumnNames failed: %v", err)
	}

	expected := []string{"id", "uuid"}
	if !reflect.DeepEqual(keys, expected) {
		t.Fatalf("Expected %v, got %v", expected, keys)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %s", err)
	}
}

func TestMySQL_SetProvider(t *testing.T) {
	db := &MySQL{}
	db.SetProvider(DriverMySQL)

	if db.Provider != DriverMySQL {
		t.Fatalf("SetProvider failed: got %q, expected %q", db.Provider, DriverMySQL)
	}
}

func TestMySQL_GetProvider(t *testing.T) {
	db := &MySQL{Provider: DriverMySQL}

	provider := db.GetProvider()
	if provider != DriverMySQL {
		t.Fatalf("GetProvider failed: got %q, expected %q", provider, DriverMySQL)
	}
}

func TestMySQL_formatTableName(t *testing.T) {
	db := &MySQL{}

	tableName := db.formatTableName(testDBNameMySQL, testDBTableNameMySQL)
	expectedTableName := fmt.Sprintf("`%s`.`%s`", testDBNameMySQL, testDBTableNameMySQL)

	if tableName != expectedTableName {
		t.Fatalf("formatTableName failed: got %q, expected %q", tableName, expectedTableName)
	}
}
