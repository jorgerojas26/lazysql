package drivers

import (
	"context"

	"github.com/jorgerojas26/lazysql/models"
)

// PageResult is one visible Records page and its lookahead state.
type PageResult struct {
	Rows        [][]string
	Query       string
	HasNextPage bool
}

// BulkTableColumnLoader is an optional driver capability for loading column
// metadata for several tables with one efficient catalog operation. Drivers
// that cannot provide a useful bulk operation should omit this capability; the
// shared schema loader will keep those tables lazy.
type BulkTableColumnLoader interface {
	GetTableColumnsBulk(database string, tables []string) (map[string][][]string, error)
}

type Driver interface {
	Connect(urlstr string) error
	TestConnection(urlstr string) error
	GetDatabases() ([]string, error)
	GetTables(database string) (map[string][]string, error)
	GetTableColumns(database, table string) ([][]string, error)
	GetConstraints(database, table string) ([][]string, error)
	GetForeignKeys(ctx context.Context, database, table string) ([][]string, error)
	GetIndexes(database, table string) ([][]string, error)
	GetRecords(ctx context.Context, database, table, where, sort string, offset, limit int) (PageResult, error)
	GetEstimatedRowCount(ctx context.Context, database, table string) (*int64, error)
	GetExactRowCount(ctx context.Context, database, table, where string) (int64, error)
	UpdateRecord(database, table, column, value, primaryKeyColumnName, primaryKeyValue string) error
	DeleteRecord(database, table string, primaryKeyColumnName, primaryKeyValue string) error
	ExecuteDMLStatement(query string) (string, error)
	ExecuteQuery(query string) ([][]string, int, error)
	ExecutePendingChanges(changes []models.DBDMLChange) error
	GetProvider() string
	GetPrimaryKeyColumnNames(database, table string) ([]string, error)

	SupportsProgramming() bool
	UseSchemas() bool
	GetFunctions(database string) (map[string][]string, error)
	GetProcedures(database string) (map[string][]string, error)
	GetViews(database string) (map[string][]string, error)
	GetFunctionDefinition(database string, name string) (string, error)
	GetProcedureDefinition(database string, name string) (string, error)
	GetViewDefinition(database string, name string) (string, error)

	FormatArg(arg any, colype models.CellValueType) any
	FormatArgForQueryString(arg any) string
	FormatReference(reference string) string
	FormatPlaceholder(index int) string

	// This converts a DML change to a query string with arg values
	DMLChangeToQueryString(change models.DBDMLChange) (string, error)

	// NOTE: This is used to get the primary key from the database table until I
	// find a better way to do it. See *ResultsTable.GetPrimaryKeyValue()
	SetProvider(provider string)
}
