package drivers

import (
	"github.com/jorgerojas26/lazysql/models"
)

type Driver interface {
	Connect(urlstr string) error
	TestConnection(urlstr string) error
	GetDatabases() ([]string, error)
	GetTables(database string) (map[string][]string, error)
	GetTableColumns(database, table string) ([][]string, error)
	GetConstraints(database, table string) ([][]string, error)
	GetForeignKeys(database, table string) ([][]string, error)
	GetIndexes(database, table string) ([][]string, error)
	GetRecords(database, table, where, sort string, offset, limit int) ([][]string, int, string, error)
	UpdateRecord(database, table, column, value, primaryKeyColumnName, primaryKeyValue string) error
	DeleteRecord(database, table string, primaryKeyColumnName, primaryKeyValue string) error
	ExecuteDMLStatement(query string) (string, error)
	// database selects which database the query runs against. Drivers that
	// support switching database context within a single connection (MSSQL,
	// MySQL) prefix the query accordingly; drivers that require a dedicated
	// connection per database (PostgreSQL) route the query through it.
	// An empty database falls back to the connection's current database.
	ExecuteQuery(database, query string) ([][]string, int, error)
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
