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
	GetTableColumnsBulk(ctx context.Context, database string, tables []string) (map[string][][]string, error)
}

// contextOrBackground keeps the final Driver contract safe for callers that
// have no operation-specific parent context while ensuring every database/sql
// call still goes through a context-aware API.
func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

type Driver interface {
	Connect(ctx context.Context, urlstr string) error
	TestConnection(ctx context.Context, urlstr string) error
	GetDatabases(ctx context.Context) ([]string, error)
	GetTables(ctx context.Context, database string) (map[string][]string, error)
	GetTableColumns(ctx context.Context, database, table string) ([][]string, error)
	GetConstraints(ctx context.Context, database, table string) ([][]string, error)
	GetForeignKeys(ctx context.Context, database, table string) ([][]string, error)
	GetIndexes(ctx context.Context, database, table string) ([][]string, error)
	GetRecords(ctx context.Context, database, table, where, sort string, offset, limit int) (PageResult, error)
	GetEstimatedRowCount(ctx context.Context, database, table string) (*int64, error)
	GetExactRowCount(ctx context.Context, database, table, where string) (int64, error)
	UpdateRecord(ctx context.Context, database, table, column, value, primaryKeyColumnName, primaryKeyValue string) error
	DeleteRecord(ctx context.Context, database, table string, primaryKeyColumnName, primaryKeyValue string) error
	ExecuteDMLStatement(ctx context.Context, query string) (string, error)
	ExecuteQuery(ctx context.Context, query string) ([][]string, int, error)
	ExecutePendingChanges(ctx context.Context, changes []models.DBDMLChange) error
	GetProvider() string
	GetPrimaryKeyColumnNames(ctx context.Context, database, table string) ([]string, error)

	SupportsProgramming() bool
	UseSchemas() bool
	GetFunctions(ctx context.Context, database string) (map[string][]string, error)
	GetProcedures(ctx context.Context, database string) (map[string][]string, error)
	GetViews(ctx context.Context, database string) (map[string][]string, error)
	GetFunctionDefinition(ctx context.Context, database string, name string) (string, error)
	GetProcedureDefinition(ctx context.Context, database string, name string) (string, error)
	GetViewDefinition(ctx context.Context, database string, name string) (string, error)

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

var (
	_ Driver                = (*MySQL)(nil)
	_ Driver                = (*Postgres)(nil)
	_ Driver                = (*MSSQL)(nil)
	_ Driver                = (*SQLite)(nil)
	_ BulkTableColumnLoader = (*MySQL)(nil)
	_ BulkTableColumnLoader = (*Postgres)(nil)
	_ BulkTableColumnLoader = (*MSSQL)(nil)
	_ BulkTableColumnLoader = (*SQLite)(nil)
)
