package drivers

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	gomock "github.com/DATA-DOG/go-sqlmock"

	"github.com/jorgerojas26/lazysql/models"
)

// mockDriver implements Driver with postgres-like formatting for unit tests.
type mockDriver struct{}

func (m *mockDriver) Connect(string) error                              { panic("not used") }
func (m *mockDriver) TestConnection(string) error                       { panic("not used") }
func (m *mockDriver) GetDatabases() ([]string, error)                   { panic("not used") }
func (m *mockDriver) GetTables(string) (map[string][]string, error)     { panic("not used") }
func (m *mockDriver) GetTableColumns(string, string) ([][]string, error) { panic("not used") }
func (m *mockDriver) GetConstraints(string, string) ([][]string, error) { panic("not used") }
func (m *mockDriver) GetForeignKeys(string, string) ([][]string, error) { panic("not used") }
func (m *mockDriver) GetIndexes(string, string) ([][]string, error)     { panic("not used") }
func (m *mockDriver) GetRecords(string, string, string, string, int, int) ([][]string, int, string, error) {
	panic("not used")
}
func (m *mockDriver) UpdateRecord(string, string, string, string, string, string) error {
	panic("not used")
}
func (m *mockDriver) DeleteRecord(string, string, string, string) error   { panic("not used") }
func (m *mockDriver) ExecuteDMLStatement(string) (string, error)          { panic("not used") }
func (m *mockDriver) ExecuteQuery(string) ([][]string, int, error)        { panic("not used") }
func (m *mockDriver) ExecutePendingChanges([]models.DBDMLChange) error    { panic("not used") }
func (m *mockDriver) GetProvider() string                                 { return "mock" }
func (m *mockDriver) GetPrimaryKeyColumnNames(string, string) ([]string, error) {
	panic("not used")
}
func (m *mockDriver) SupportsProgramming() bool                                  { return false }
func (m *mockDriver) UseSchemas() bool                                           { return false }
func (m *mockDriver) GetFunctions(string) (map[string][]string, error)           { panic("not used") }
func (m *mockDriver) GetProcedures(string) (map[string][]string, error)          { panic("not used") }
func (m *mockDriver) GetViews(string) (map[string][]string, error)               { panic("not used") }
func (m *mockDriver) GetFunctionDefinition(string, string) (string, error)       { panic("not used") }
func (m *mockDriver) GetProcedureDefinition(string, string) (string, error)      { panic("not used") }
func (m *mockDriver) GetViewDefinition(string, string) (string, error)           { panic("not used") }
func (m *mockDriver) DMLChangeToQueryString(models.DBDMLChange) (string, error)  { panic("not used") }
func (m *mockDriver) SetProvider(string)                                         {}

func (m *mockDriver) FormatArg(arg any, _ models.CellValueType) any {
	return arg
}

func (m *mockDriver) FormatArgForQueryString(arg any) string {
	switch v := arg.(type) {
	case string:
		return fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''"))
	case nil:
		return "NULL"
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (m *mockDriver) FormatReference(reference string) string {
	return fmt.Sprintf("\"%s\"", reference)
}

func (m *mockDriver) FormatPlaceholder(index int) string {
	return fmt.Sprintf("$%d", index)
}

func Test_queriesInTransaction(t *testing.T) {
	tests := []struct {
		setMockExpectations func(mock gomock.Sqlmock)
		assertErr           func(t *testing.T, err error)
		name                string
		queries             []models.Query
	}{
		{
			name: "successful transaction",
			queries: []models.Query{
				{Query: "SELECT * FROM table"},
			},
			setMockExpectations: func(mock gomock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("SELECT \\* FROM table").WillReturnResult(gomock.NewResult(0, 0))
				mock.ExpectCommit()
			},
		},
		{
			name: "unsuccessful commit",
			queries: []models.Query{
				{Query: "SELECT * FROM table"},
			},
			setMockExpectations: func(mock gomock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("SELECT \\* FROM table").WillReturnResult(gomock.NewResult(0, 0))
				mock.ExpectCommit().WillReturnError(errors.New("commit error"))
			},
			assertErr: func(t *testing.T, err error) {
				t.Helper()
				if !strings.Contains(err.Error(), "commit error") {
					t.Errorf("expected error to contain 'commit error', got %v", err)
				}
			},
		},
		{
			name: "failed query",
			queries: []models.Query{
				{Query: "SELECT * FROM table"},
			},
			setMockExpectations: func(mock gomock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("SELECT \\* FROM table").WillReturnError(errors.New("query error"))
				mock.ExpectRollback()
			},
			assertErr: func(t *testing.T, err error) {
				t.Helper()
				if !strings.Contains(err.Error(), "query error") {
					t.Errorf("expected error to contain 'commit error', got %v", err)
				}
			},
		},
		{
			name: "failed 2nd query of three",
			queries: []models.Query{
				{Query: "SELECT * FROM table"},
				{Query: "SELECT * FROM table"},
				{Query: "SELECT * FROM table"},
			},
			setMockExpectations: func(mock gomock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("SELECT \\* FROM table").WillReturnResult(gomock.NewResult(0, 0))
				mock.ExpectExec("SELECT \\* FROM table").WillReturnError(errors.New("query error"))
				mock.ExpectRollback()
			},
			assertErr: func(t *testing.T, err error) {
				t.Helper()
				if !strings.Contains(err.Error(), "query error") {
					t.Errorf("expected error to contain 'commit error', got %v", err)
				}
			},
		},
		{
			name: "failed query and rollback",
			queries: []models.Query{
				{Query: "SELECT * FROM table"},
			},
			setMockExpectations: func(mock gomock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectExec("SELECT \\* FROM table").WillReturnError(errors.New("query error"))
				mock.ExpectRollback().WillReturnError(errors.New("rollback error"))
			},
			assertErr: func(t *testing.T, err error) {
				t.Helper()
				errMsg := err.Error()
				if !strings.Contains(errMsg, "query error") {
					t.Errorf("expected error to contain 'commit error', got %v", err)
				}
				if !strings.Contains(errMsg, "rollback error") {
					t.Errorf("expected error to contain 'rollback error', got %v", err)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := gomock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			tt.setMockExpectations(mock)
			queryErr := queriesInTransaction(db, tt.queries)
			if tt.assertErr != nil {
				tt.assertErr(t, queryErr)
			}
		})
	}
}

func Test_buildUpdateQuery(t *testing.T) {
	d := &mockDriver{}

	tests := []struct {
		name           string
		values         []models.CellValue
		primaryKeyInfo []models.PrimaryKeyInfo
		wantQuery      string
		wantArgs       []any
	}{
		{
			name: "single PK column",
			values: []models.CellValue{
				{Column: "name", Value: "Alice", Type: models.String},
			},
			primaryKeyInfo: []models.PrimaryKeyInfo{
				{Name: "id", Value: 1},
			},
			wantQuery: `UPDATE "test_table" SET "name" = $1 WHERE "id" = $2`,
			wantArgs:  []any{"Alice", 1},
		},
		{
			name: "multiple PK columns",
			values: []models.CellValue{
				{Column: "name", Value: "Alice", Type: models.String},
			},
			primaryKeyInfo: []models.PrimaryKeyInfo{
				{Name: "id", Value: 1},
				{Name: "tenant", Value: "acme"},
			},
			wantQuery: `UPDATE "test_table" SET "name" = $1 WHERE "id" = $2 AND "tenant" = $3`,
			wantArgs:  []any{"Alice", 1, "acme"},
		},
		{
			name:           "empty PKI returns empty query",
			values:         []models.CellValue{{Column: "name", Value: "x", Type: models.String}},
			primaryKeyInfo: []models.PrimaryKeyInfo{},
			wantQuery:      "",
			wantArgs:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildUpdateQuery(`"test_table"`, tt.values, tt.primaryKeyInfo, d)
			if got.Query != tt.wantQuery {
				t.Errorf("query mismatch:\n  got:  %s\n  want: %s", got.Query, tt.wantQuery)
			}
			if !reflect.DeepEqual(got.Args, tt.wantArgs) {
				t.Errorf("args mismatch:\n  got:  %v\n  want: %v", got.Args, tt.wantArgs)
			}
		})
	}
}

func Test_buildDeleteQuery(t *testing.T) {
	d := &mockDriver{}

	tests := []struct {
		name           string
		primaryKeyInfo []models.PrimaryKeyInfo
		wantQuery      string
		wantArgs       []any
	}{
		{
			name: "single PK column",
			primaryKeyInfo: []models.PrimaryKeyInfo{
				{Name: "id", Value: 1},
			},
			wantQuery: `DELETE FROM "test_table" WHERE "id" = $1`,
			wantArgs:  []any{1},
		},
		{
			name: "multiple PK columns",
			primaryKeyInfo: []models.PrimaryKeyInfo{
				{Name: "id", Value: 1},
				{Name: "tenant", Value: "acme"},
			},
			wantQuery: `DELETE FROM "test_table" WHERE "id" = $1 AND "tenant" = $2`,
			wantArgs:  []any{1, "acme"},
		},
		{
			name:           "empty PKI returns empty query",
			primaryKeyInfo: []models.PrimaryKeyInfo{},
			wantQuery:      "",
			wantArgs:       nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDeleteQuery(`"test_table"`, tt.primaryKeyInfo, d)
			if got.Query != tt.wantQuery {
				t.Errorf("query mismatch:\n  got:  %s\n  want: %s", got.Query, tt.wantQuery)
			}
			if !reflect.DeepEqual(got.Args, tt.wantArgs) {
				t.Errorf("args mismatch:\n  got:  %v\n  want: %v", got.Args, tt.wantArgs)
			}
		})
	}
}

func Test_buildDeleteQueryString(t *testing.T) {
	d := &mockDriver{}

	tests := []struct {
		name           string
		primaryKeyInfo []models.PrimaryKeyInfo
		wantQuery      string
	}{
		{
			name: "single PK column",
			primaryKeyInfo: []models.PrimaryKeyInfo{
				{Name: "id", Value: 1},
			},
			wantQuery: `DELETE FROM "test_table" WHERE "id" = 1`,
		},
		{
			name: "multiple PK columns",
			primaryKeyInfo: []models.PrimaryKeyInfo{
				{Name: "id", Value: 1},
				{Name: "tenant", Value: "acme"},
			},
			wantQuery: `DELETE FROM "test_table" WHERE "id" = 1 AND "tenant" = 'acme'`,
		},
		{
			name:           "empty PKI produces no WHERE clause",
			primaryKeyInfo: []models.PrimaryKeyInfo{},
			wantQuery:      `DELETE FROM "test_table"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDeleteQueryString(`"test_table"`, tt.primaryKeyInfo, d)
			if got != tt.wantQuery {
				t.Errorf("query mismatch:\n  got:  %s\n  want: %s", got, tt.wantQuery)
			}
		})
	}
}
