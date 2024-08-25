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
	GetRecords(database, table, where, sort string, offset, limit int) ([][]string, int, error)
	UpdateRecord(database, table, column, value, primaryKeyColumnName, primaryKeyValue string) error
	DeleteRecord(database, table string, primaryKeyColumnName, primaryKeyValue string) error
	ExecuteDMLStatement(query string) (string, error)
	ExecuteQuery(query string) ([][]string, error)
	ExecutePendingChanges(changes []models.DbDmlChange) error
	SetProvider(provider string) // NOTE: This is used to get the primary key from the database table until i find a better way to do it. See ResultsTable.go GetPrimaryKeyValue function
	GetProvider() string
}
