package drivers

import (
	"github.com/jorgerojas26/lazysql/models"
)

type Driver interface {
	Connect(urlstr string) error
	TestConnection(urlstr string) error
	GetDatabases() ([]string, error)
	GetTables(database string) ([]string, error)
	GetTableColumns(database, table string) ([][]string, error)
	GetConstraints(table string) ([][]string, error)
	GetForeignKeys(table string) ([][]string, error)
	GetIndexes(table string) ([][]string, error)
	GetRecords(table, where, sort string, offset, limit int) ([][]string, int, error)
	UpdateRecord(table, column, value, id string) error
	DeleteRecord(table string, id string) error
	ExecuteDMLStatement(query string) (string, error)
	ExecuteQuery(query string) ([][]string, error)
	ExecutePendingChanges(changes []models.DbDmlChange, inserts []models.DbInsert) error
}
