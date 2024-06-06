package drivers

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMySql(t *testing.T) {
	dsn := mustGetDsn(t, "MYSQL_DSN")

	var testDatabase = "mysql"
	var testTable = "db" // not to get confused: "db" is a table name in "mysql" database, not a database name

	t.Run("Test Connect", func(t *testing.T) {
		m := &MySQL{}
		require.NoError(t, m.Connect(dsn))
		m.Connection.Close()
	})

	t.Run("Test GetDabases", func(t *testing.T) {
		m := &MySQL{}
		require.NoError(t, m.Connect(dsn))
		defer m.Connection.Close()

		databases, err := m.GetDatabases()
		require.NoError(t, err)
		require.NotEmpty(t, databases)
	})

	t.Run("Test GetTables", func(t *testing.T) {
		m := &MySQL{}
		require.NoError(t, m.Connect(dsn))
		defer m.Connection.Close()

		tables, err := m.GetTables(testDatabase)
		require.NoError(t, err)
		require.NotEmpty(t, tables)
	})

	t.Run("Test GetTableColumns", func(t *testing.T) {
		m := &MySQL{}
		require.NoError(t, m.Connect(dsn))
		defer m.Connection.Close()

		columns, err := m.GetTableColumns(testDatabase, testTable)
		require.NoError(t, err)
		require.NotEmpty(t, columns)
	})

	t.Run("Test GetConstraints", func(t *testing.T) {
		m := &MySQL{}
		require.NoError(t, m.Connect(dsn))
		defer m.Connection.Close()

		constraints, err := m.GetConstraints(testDatabase + "." + testTable)
		require.NoError(t, err)
		require.NotEmpty(t, constraints)
	})

	t.Run("Test GetForeignKeys", func(t *testing.T) {
		m := &MySQL{}
		require.NoError(t, m.Connect(dsn))
		defer m.Connection.Close()

		foreignKeys, err := m.GetForeignKeys(testDatabase + "." + testTable)
		require.NoError(t, err)
		require.NotEmpty(t, foreignKeys)
	})

	t.Run("Test GetIndexes", func(t *testing.T) {
		m := &MySQL{}
		require.NoError(t, m.Connect(dsn))
		defer m.Connection.Close()

		indexes, err := m.GetIndexes(testTable)
		require.NoError(t, err)
		require.NotEmpty(t, indexes)
	})

	t.Run("Test GetRecords", func(t *testing.T) {
		m := &MySQL{}
		require.NoError(t, m.Connect(dsn))
		defer m.Connection.Close()

		records, totalRecords, err := m.GetRecords(testTable, "", "", 0, 5)
		require.NoError(t, err)
		require.Len(t, records, 5)
		require.NotZero(t, totalRecords)
	})

	t.Run("Test ExecuteQuery", func(t *testing.T) {
		m := &MySQL{}
		require.NoError(t, m.Connect(dsn))
		defer m.Connection.Close()

		res, err := m.ExecuteQuery(fmt.Sprintf("SELECT * FROM %s LIMIT 5", testTable))
		require.NoError(t, err)
		require.Len(t, res, 5)
	})

	t.Run("Test UpdateRecord", func(t *testing.T) {
		t.Skip("This is a pretty setup-specific test, I would recommend to trigger manually until there is a proper/universal test setup")
		m := &MySQL{}
		require.NoError(t, m.Connect(dsn))
		defer m.Connection.Close()

		err := m.UpdateRecord(testTable, "columnName", "newValue", "id", "1")
		require.NoError(t, err)
	})

	t.Run("Test DeleteRecord", func(t *testing.T) {
		t.Skip("This is a pretty setup-specific test, I would recommend to trigger manually until there is a proper/universal test setup")
		m := &MySQL{}
		require.NoError(t, m.Connect(dsn))
		defer m.Connection.Close()

		err := m.DeleteRecord(testTable, "id", "1")
		require.NoError(t, err)
	})
}
