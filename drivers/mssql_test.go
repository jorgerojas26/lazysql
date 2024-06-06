package drivers

import (
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/require"
)

// mustGetDsn returns the DSN from the environment with the given argName
// for simplicity it loads the .env file from the root of the project so you can store the DSN there
func mustGetDsn(t *testing.T, argName string) string {
	t.Helper()
	require.NoError(t, godotenv.Load("../.env"))

	dsn := os.Getenv(argName)
	if dsn == "" {
		require.FailNowf(t, "MSSQL_DSN is empty", "MSSQL_DSN is empty")
	}

	return dsn
}

func TestMssql(t *testing.T) {
	dsn := mustGetDsn(t, "MSSQL_DSN")

	// this may have to be altered depending on your setup. On my prod database master has 0 tables
	testDatabase := "master"
	testTable := "testTable"

	t.Run("Test Connect to working database", func(t *testing.T) {
		db := &MsSql{}
		require.NoError(t, db.Connect(dsn))
		require.NoError(t, db.Connection.Close())
	})

	t.Run("Test Connect to non-working database", func(t *testing.T) {
		db := &MsSql{}
		require.Error(t, db.Connect("sqlserver://sa:rootpass@0.0.0.0:1433?database=testdb&app_name=lazysql"))
	})

	t.Run("Test GetDatabases from working database", func(t *testing.T) {
		db := &MsSql{}
		require.NoError(t, db.Connect(dsn))
		defer db.Connection.Close()

		databases, err := db.GetDatabases()
		require.NoError(t, err)
		require.NotEmpty(t, databases)
	})

	t.Run("Test GetTables from working database", func(t *testing.T) {
		db := &MsSql{}
		require.NoError(t, db.Connect(dsn))
		defer db.Connection.Close()

		tables, err := db.GetTables(testDatabase)
		require.NoError(t, err)
		require.NotEmpty(t, tables)
	})

	t.Run("Test GetTableColumns from working database", func(t *testing.T) {
		db := &MsSql{}
		require.NoError(t, db.Connect(dsn))
		defer db.Connection.Close()

		columns, err := db.GetTableColumns(testDatabase, testTable)
		require.NoError(t, err)
		require.NotEmpty(t, columns)
	})

	t.Run("Test GetConstraints from working database", func(t *testing.T) {
		db := &MsSql{}
		require.NoError(t, db.Connect(dsn))
		defer db.Connection.Close()

		constraints, err := db.GetConstraints(testTable)
		require.NoError(t, err)
		require.NotEmpty(t, constraints)
	})

	t.Run("Test GetForeignKeys from working database", func(t *testing.T) {
		// todo: verify this test, my prod database has no foreign keys (it's a legacy database)
		db := &MsSql{}
		require.NoError(t, db.Connect(dsn))
		defer db.Connection.Close()

		foreignKeys, err := db.GetForeignKeys(testTable)
		require.NoError(t, err)
		require.NotEmpty(t, foreignKeys)
	})

	t.Run("Test GetIndexes from working database", func(t *testing.T) {
		db := &MsSql{}
		require.NoError(t, db.Connect(dsn))
		defer db.Connection.Close()

		indexes, err := db.GetIndexes(testTable)
		require.NoError(t, err)
		require.NotEmpty(t, indexes)
	})

	t.Run("Test GetRecords from working database", func(t *testing.T) {
		db := &MsSql{}
		require.NoError(t, db.Connect(dsn))
		defer db.Connection.Close()

		records, count, err := db.GetRecords(testTable, "", "", 0, 10)
		require.NoError(t, err)
		require.NotZero(t, count)
		require.Len(t, records, 10)
	})

	t.Run("Test ExecuteQuery from working database", func(t *testing.T) {
		db := &MsSql{}
		require.NoError(t, db.Connect(dsn))
		defer db.Connection.Close()

		rows, err := db.ExecuteQuery("SELECT TOP 10 * FROM " + testTable)
		require.NoError(t, err)
		require.Len(t, rows, 10)
	})

	t.Run("Test UpdateRecord from working database", func(t *testing.T) {
		t.Skip("pretty specific to the database in question. Since it is hard to provide a generic test for this, I'm skipping it. It worked on my machine though")
		db := &MsSql{}
		require.NoError(t, db.Connect(dsn))
		defer db.Connection.Close()

		err := db.UpdateRecord(testTable, "testCol", "newVal", "id", "1")
		require.NoError(t, err)
	})

	t.Run("Test ExecuteDMLStatement from working database", func(t *testing.T) {
		db := &MsSql{}
		require.NoError(t, db.Connect(dsn))
		defer db.Connection.Close()

		res, err := db.ExecuteDMLStatement("SELECT TOP 10 * FROM " + testTable)
		require.NoError(t, err)
		require.Equal(t, "10 rows affected", res)
	})
}
