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
	GetConstraints(table string) ([][]string, error)
	GetForeignKeys(table string) ([][]string, error)
	GetIndexes(table string) ([][]string, error)
	GetRecords(table, where, sort string, offset, limit int) ([][]string, int, error)
	UpdateRecord(table, column, value, primaryKeyColumnName, primaryKeyValue string) error
	DeleteRecord(table string, primaryKeyColumnName, primaryKeyValue string) error
	ExecuteDMLStatement(query string) (string, error)
	ExecuteQuery(query string) ([][]string, error)
	ExecutePendingChanges(changes []models.DbDmlChange) error
	SetProvider(provider string) // NOTE: This is used to get the primary key from the database table until i find a better way to do it. See ResultsTable.go GetPrimaryKeyValue function
	GetProvider() string
}
