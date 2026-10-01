package drivers

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/jorgerojas26/lazysql/models"
)

// TestClickHouse_Integration runs the ClickHouse driver against a real server.
// It is skipped unless LAZYSQL_CLICKHOUSE_URL is set, for example:
//
//	docker run -d -p 19000:9000 -e CLICKHOUSE_PASSWORD=pass clickhouse/clickhouse-server
//	LAZYSQL_CLICKHOUSE_URL=clickhouse://default:pass@localhost:19000/default go test ./drivers -run ClickHouse_Integration
func TestClickHouse_Integration(t *testing.T) {
	url := os.Getenv("LAZYSQL_CLICKHOUSE_URL")
	if url == "" {
		t.Skip("LAZYSQL_CLICKHOUSE_URL is not set")
	}

	const database = "lazysql_integration"

	db := &ClickHouse{}
	if err := db.TestConnection(context.Background(), url); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if err := db.Connect(context.Background(), url); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer db.Connection.Close()

	if db.GetProvider() != DriverClickHouse {
		t.Fatalf("expected provider %q, got %q", DriverClickHouse, db.GetProvider())
	}

	setup := []string{
		"DROP DATABASE IF EXISTS " + database,
		"CREATE DATABASE " + database,
		`CREATE TABLE ` + database + `.events (
			id UInt64,
			day Date,
			created DateTime64(3, 'UTC'),
			name String,
			note Nullable(String),
			tags Array(String),
			attrs Map(String, UInt32),
			score Float64 DEFAULT 1.5,
			INDEX name_idx name TYPE bloom_filter GRANULARITY 1
		) ENGINE = MergeTree PARTITION BY toYYYYMM(day) ORDER BY (id, day)`,
		"CREATE TABLE " + database + ".logs (msg String) ENGINE = TinyLog",
		`INSERT INTO ` + database + `.events (id, day, created, name, note, tags, attrs) VALUES
			(1, '2024-01-01', '2024-01-01 10:00:00.123', 'alpha', NULL, ['a', 'b'], {'k': 1}),
			(2, '2024-01-02', '2024-01-02 11:00:00', 'beta', 'n2', [], {}),
			(3, '2024-02-03', '2024-02-03 12:00:00', '', 'n3', ['c'], {'x': 2})`,
	}
	for _, query := range setup {
		if _, err := db.Connection.Exec(query); err != nil {
			t.Fatalf("setup %q: %v", query, err)
		}
	}
	defer db.Connection.Exec("DROP DATABASE IF EXISTS " + database) //nolint:errcheck

	databases, err := db.GetDatabases(context.Background())
	if err != nil {
		t.Fatalf("GetDatabases: %v", err)
	}
	if !contains(databases, database) || contains(databases, "system") {
		t.Fatalf("GetDatabases: unexpected result %v", databases)
	}

	tables, err := db.GetTables(context.Background(), database)
	if err != nil {
		t.Fatalf("GetTables: %v", err)
	}
	if !reflect.DeepEqual(tables[database], []string{"events", "logs"}) {
		t.Fatalf("GetTables: unexpected result %v", tables)
	}

	columns, err := db.GetTableColumns(context.Background(), database, "events")
	if err != nil {
		t.Fatalf("GetTableColumns: %v", err)
	}
	if len(columns) != 9 || columns[1][0] != "id" || columns[1][1] != "UInt64" || columns[8][0] != "score" {
		t.Fatalf("GetTableColumns: unexpected result %v", columns)
	}

	constraints, err := db.GetConstraints(context.Background(), database, "events")
	if err != nil {
		t.Fatalf("GetConstraints: %v", err)
	}
	if len(constraints) != 2 || constraints[1][0] != "MergeTree" || constraints[1][1] != "toYYYYMM(day)" || constraints[1][2] != "id, day" {
		t.Fatalf("GetConstraints: unexpected result %v", constraints)
	}

	foreignKeys, err := db.GetForeignKeys(context.Background(), database, "events")
	if err != nil || len(foreignKeys) != 1 {
		t.Fatalf("GetForeignKeys: unexpected result %v, %v", foreignKeys, err)
	}

	indexes, err := db.GetIndexes(context.Background(), database, "events")
	if err != nil {
		t.Fatalf("GetIndexes: %v", err)
	}
	if len(indexes) != 2 || indexes[1][0] != "name_idx" || indexes[1][1] != "bloom_filter" {
		t.Fatalf("GetIndexes: unexpected result %v", indexes)
	}

	primaryKeys, err := db.GetPrimaryKeyColumnNames(context.Background(), database, "events")
	if err != nil {
		t.Fatalf("GetPrimaryKeyColumnNames: %v", err)
	}
	if !reflect.DeepEqual(primaryKeys, []string{"id", "day"}) {
		t.Fatalf("GetPrimaryKeyColumnNames: unexpected result %v", primaryKeys)
	}

	noPrimaryKeys, err := db.GetPrimaryKeyColumnNames(context.Background(), database, "logs")
	if err != nil || len(noPrimaryKeys) != 0 {
		t.Fatalf("GetPrimaryKeyColumnNames(logs): unexpected result %v, %v", noPrimaryKeys, err)
	}

	page, err := db.GetRecords(context.Background(), database, "events", "", "id ASC", 0, 0)
	if err != nil {
		t.Fatalf("GetRecords: %v", err)
	}
	records := page.Rows
	if page.HasNextPage || len(records) != 4 {
		t.Fatalf("GetRecords: expected 3 records, got total %d, rows %v (query %s)", len(records)-1, records, page.Query)
	}
	expectedFirst := []string{"1", "2024-01-01", "2024-01-01 10:00:00.123", "alpha", "NULL&", "['a','b']", "{'k':1}", "1.5"}
	if !reflect.DeepEqual(records[1], expectedFirst) {
		t.Fatalf("GetRecords: expected first row %v, got %v", expectedFirst, records[1])
	}
	if records[3][3] != "EMPTY&" {
		t.Fatalf("GetRecords: expected EMPTY& for empty string, got %q", records[3][3])
	}

	page, err = db.GetRecords(context.Background(), database, "events", "WHERE id > 1", "id DESC", 1, 1)
	if err != nil {
		t.Fatalf("GetRecords (filtered): %v", err)
	}
	records = page.Rows
	if page.HasNextPage || len(records) != 2 || records[1][0] != "2" {
		t.Fatalf("GetRecords (filtered): unexpected total %d, rows %v", len(records)-1, records)
	}

	var streamed [][]string
	streamResult, streamErr := db.StreamQuery(context.Background(), "", "SELECT id, note, tags, attrs FROM "+database+".events ORDER BY id", 2, func(batch QueryBatch) error { streamed = append(streamed, batch.Rows...); return nil })
	if streamErr != nil || !streamResult.Truncated || len(streamed) != 2 || !reflect.DeepEqual(streamed[0], []string{"1", "NULL", "['a','b']", "{'k':1}"}) {
		t.Fatalf("typed stream = %v, %+v, %v", streamed, streamResult, streamErr)
	}

	results, count, err := db.ExecuteQuery(context.Background(), "", "SELECT id, note FROM "+database+".events ORDER BY id")
	if err != nil {
		t.Fatalf("ExecuteQuery: %v", err)
	}
	if count != 3 || results[1][1] != "NULL" || results[2][1] != "n2" {
		t.Fatalf("ExecuteQuery: unexpected result %v", results)
	}

	if _, err := db.ExecuteDMLStatement(context.Background(), "", "INSERT INTO "+database+".logs VALUES ('hello')"); err != nil {
		t.Fatalf("ExecuteDMLStatement: %v", err)
	}

	changes := []models.DBDMLChange{
		{
			Type:     models.DMLInsertType,
			Database: database,
			Table:    "events",
			Values: []models.CellValue{
				{Column: "id", Value: "4", Type: models.String},
				{Column: "day", Value: "2024-03-04", Type: models.String},
				{Column: "created", Value: "2024-03-04 00:00:00", Type: models.String},
				{Column: "name", Value: "delta", Type: models.String},
				{Column: "score", Value: "DEFAULT", Type: models.Default},
			},
		},
		{
			Type:     models.DMLUpdateType,
			Database: database,
			Table:    "events",
			Values: []models.CellValue{
				{Column: "name", Value: "alpha'updated", Type: models.String},
				{Column: "note", Value: "NULL", Type: models.Null},
				{Column: "attrs", Value: "{'updated':2}", Type: models.String},
			},
			PrimaryKeyInfo: []models.PrimaryKeyInfo{{Name: "id", Value: "1"}, {Name: "day", Value: "2024-01-01"}},
		},
		{
			Type:           models.DMLDeleteType,
			Database:       database,
			Table:          "events",
			PrimaryKeyInfo: []models.PrimaryKeyInfo{{Name: "id", Value: "2"}, {Name: "day", Value: "2024-01-02"}},
		},
	}
	for _, change := range changes {
		if _, err := db.DMLChangeToQueryString(change); err != nil {
			t.Fatalf("DMLChangeToQueryString: %v", err)
		}
	}
	if err := db.ExecutePendingChanges(context.Background(), changes); err != nil {
		t.Fatalf("ExecutePendingChanges: %v", err)
	}

	results, _, err = db.ExecuteQuery(context.Background(), "", "SELECT id, name, note IS NULL AS note_is_null, score, attrs FROM "+database+".events ORDER BY id")
	if err != nil {
		t.Fatalf("ExecuteQuery after changes: %v", err)
	}
	expected := [][]string{
		{"id", "name", "note_is_null", "score", "attrs"},
		{"1", "alpha'updated", "1", "1.5", "{'updated':2}"},
		{"3", "", "0", "1.5", "{'x':2}"},
		{"4", "delta", "1", "1.5", "{}"},
	}
	if !reflect.DeepEqual(results, expected) {
		t.Fatalf("ExecutePendingChanges: expected %v, got %v", expected, results)
	}

	// The query preview must be valid ClickHouse SQL as well.
	preview, err := db.DMLChangeToQueryString(models.DBDMLChange{
		Type:           models.DMLUpdateType,
		Database:       database,
		Table:          "events",
		Values:         []models.CellValue{{Column: "name", Value: `back\slash`, Type: models.String}},
		PrimaryKeyInfo: []models.PrimaryKeyInfo{{Name: "id", Value: "3"}},
	})
	if err != nil {
		t.Fatalf("DMLChangeToQueryString: %v", err)
	}
	if _, err := db.Connection.ExecContext(clickHouseMutationContext(context.Background()), preview); err != nil {
		t.Fatalf("executing preview %q: %v", preview, err)
	}

	if err := db.UpdateRecord(context.Background(), database, "events", "name", "via UpdateRecord", "id", "4"); err != nil {
		t.Fatalf("UpdateRecord: %v", err)
	}
	if err := db.DeleteRecord(context.Background(), database, "events", "id", "1"); err != nil {
		t.Fatalf("DeleteRecord: %v", err)
	}

	results, _, err = db.ExecuteQuery(context.Background(), "", "SELECT id, name FROM "+database+".events ORDER BY id")
	if err != nil {
		t.Fatalf("ExecuteQuery after UpdateRecord/DeleteRecord: %v", err)
	}
	expected = [][]string{
		{"id", "name"},
		{"3", `back\slash`},
		{"4", "via UpdateRecord"},
	}
	if !reflect.DeepEqual(results, expected) {
		t.Fatalf("UpdateRecord/DeleteRecord: expected %v, got %v", expected, results)
	}

	complexResults, _, err := db.ExecuteQuery(context.Background(), "", `SELECT
		[toDate('2024-01-02')] AS dates,
		[toUUID('61f0c404-5cb3-11e7-907b-a6006ad3dba0')] AS ids,
		CAST((2, 'x') AS Nullable(Tuple(Int32, String))) AS nullable_tuple,
		[(3, 'y')]::Array(Tuple(Int32, String)) AS tuples`)
	if err != nil {
		t.Fatalf("ExecuteQuery complex values: %v", err)
	}
	complexExpected := [][]string{
		{"dates", "ids", "nullable_tuple", "tuples"},
		{"['2024-01-02']", "['61f0c404-5cb3-11e7-907b-a6006ad3dba0']", "(2,'x')", "[(3,'y')]"},
	}
	if !reflect.DeepEqual(complexResults, complexExpected) {
		t.Fatalf("complex value formatting: expected %v, got %v", complexExpected, complexResults)
	}

	oddDatabase, oddTable, oddColumn := "lazysql`odd", "t`x", "c`x"
	oddTableName := db.formatTableName(oddDatabase, oddTable)
	for _, query := range []string{
		"CREATE DATABASE " + db.FormatReference(oddDatabase),
		"CREATE TABLE " + oddTableName + " (`id` UInt8, " + db.FormatReference(oddColumn) + " String) ENGINE=MergeTree ORDER BY id",
		"INSERT INTO " + oddTableName + " VALUES (1, 'before')",
	} {
		if _, err := db.Connection.Exec(query); err != nil {
			t.Fatalf("identifier setup %q: %v", query, err)
		}
	}
	defer db.Connection.Exec("DROP DATABASE IF EXISTS " + db.FormatReference(oddDatabase)) //nolint:errcheck
	oddPage, err := db.GetRecords(context.Background(), oddDatabase, oddTable, "", "", 0, 10)
	if err != nil || !reflect.DeepEqual(oddPage.Rows, [][]string{{"id", oddColumn}, {"1", "before"}}) {
		t.Fatalf("GetRecords with escaped identifiers: %v, %v", oddPage.Rows, err)
	}
	if err := db.UpdateRecord(context.Background(), oddDatabase, oddTable, oddColumn, "after", "id", "1"); err != nil {
		t.Fatalf("UpdateRecord with escaped identifiers: %v", err)
	}
	oddPage, err = db.GetRecords(context.Background(), oddDatabase, oddTable, "", "", 0, 10)
	if err != nil || oddPage.Rows[1][1] != "after" {
		t.Fatalf("updated escaped identifier record: %v, %v", oddPage.Rows, err)
	}

	// Mutations are not supported on the TinyLog engine. Verify that a prior
	// successful insert is reported so the caller can remove it before retrying.
	err = db.ExecutePendingChanges(context.Background(), []models.DBDMLChange{
		{
			Type:     models.DMLInsertType,
			Database: database,
			Table:    "events",
			Values: []models.CellValue{
				{Column: "id", Value: "5", Type: models.String},
				{Column: "day", Value: "2024-05-05", Type: models.String},
				{Column: "created", Value: "2024-05-05 00:00:00", Type: models.String},
				{Column: "name", Value: "partial", Type: models.String},
			},
		},
		{
			Type:           models.DMLDeleteType,
			Database:       database,
			Table:          "logs",
			PrimaryKeyInfo: []models.PrimaryKeyInfo{{Name: "msg", Value: "hello"}},
		},
	})
	var partialErr *PartialExecutionError
	if !errors.As(err, &partialErr) || partialErr.Applied != 1 || !strings.Contains(err.Error(), "TinyLog") {
		t.Fatalf("expected one applied change before TinyLog error, got %v", err)
	}
	var inserted int
	if err := db.Connection.QueryRow("SELECT count() FROM " + database + ".events WHERE id = 5").Scan(&inserted); err != nil || inserted != 1 {
		t.Fatalf("expected exactly one partially applied insert, count=%d, err=%v", inserted, err)
	}
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
