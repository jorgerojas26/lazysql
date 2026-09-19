package drivers

import (
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
	if err := db.TestConnection(url); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if err := db.Connect(url); err != nil {
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

	databases, err := db.GetDatabases()
	if err != nil {
		t.Fatalf("GetDatabases: %v", err)
	}
	if !contains(databases, database) || contains(databases, "system") {
		t.Fatalf("GetDatabases: unexpected result %v", databases)
	}

	tables, err := db.GetTables(database)
	if err != nil {
		t.Fatalf("GetTables: %v", err)
	}
	if !reflect.DeepEqual(tables[database], []string{"events", "logs"}) {
		t.Fatalf("GetTables: unexpected result %v", tables)
	}

	columns, err := db.GetTableColumns(database, "events")
	if err != nil {
		t.Fatalf("GetTableColumns: %v", err)
	}
	if len(columns) != 9 || columns[1][0] != "id" || columns[1][1] != "UInt64" || columns[8][0] != "score" {
		t.Fatalf("GetTableColumns: unexpected result %v", columns)
	}

	constraints, err := db.GetConstraints(database, "events")
	if err != nil {
		t.Fatalf("GetConstraints: %v", err)
	}
	if len(constraints) != 2 || constraints[1][0] != "MergeTree" || constraints[1][1] != "toYYYYMM(day)" || constraints[1][2] != "id, day" {
		t.Fatalf("GetConstraints: unexpected result %v", constraints)
	}

	foreignKeys, err := db.GetForeignKeys(database, "events")
	if err != nil || len(foreignKeys) != 1 {
		t.Fatalf("GetForeignKeys: unexpected result %v, %v", foreignKeys, err)
	}

	indexes, err := db.GetIndexes(database, "events")
	if err != nil {
		t.Fatalf("GetIndexes: %v", err)
	}
	if len(indexes) != 2 || indexes[1][0] != "name_idx" || indexes[1][1] != "bloom_filter" {
		t.Fatalf("GetIndexes: unexpected result %v", indexes)
	}

	primaryKeys, err := db.GetPrimaryKeyColumnNames(database, "events")
	if err != nil {
		t.Fatalf("GetPrimaryKeyColumnNames: %v", err)
	}
	if !reflect.DeepEqual(primaryKeys, []string{"id", "day"}) {
		t.Fatalf("GetPrimaryKeyColumnNames: unexpected result %v", primaryKeys)
	}

	noPrimaryKeys, err := db.GetPrimaryKeyColumnNames(database, "logs")
	if err != nil || len(noPrimaryKeys) != 0 {
		t.Fatalf("GetPrimaryKeyColumnNames(logs): unexpected result %v, %v", noPrimaryKeys, err)
	}

	records, total, query, err := db.GetRecords(database, "events", "", "id ASC", 0, 0)
	if err != nil {
		t.Fatalf("GetRecords: %v", err)
	}
	if total != 3 || len(records) != 4 {
		t.Fatalf("GetRecords: expected 3 records, got total %d, rows %v (query %s)", total, records, query)
	}
	expectedFirst := []string{"1", "2024-01-01", "2024-01-01 10:00:00.123", "alpha", "NULL&", "['a','b']", "{'k':1}", "1.5"}
	if !reflect.DeepEqual(records[1], expectedFirst) {
		t.Fatalf("GetRecords: expected first row %v, got %v", expectedFirst, records[1])
	}
	if records[3][3] != "EMPTY&" {
		t.Fatalf("GetRecords: expected EMPTY& for empty string, got %q", records[3][3])
	}

	records, total, _, err = db.GetRecords(database, "events", "WHERE id > 1", "id DESC", 1, 1)
	if err != nil {
		t.Fatalf("GetRecords (filtered): %v", err)
	}
	if total != 2 || len(records) != 2 || records[1][0] != "2" {
		t.Fatalf("GetRecords (filtered): unexpected total %d, rows %v", total, records)
	}

	results, count, err := db.ExecuteQuery("SELECT id, note FROM " + database + ".events ORDER BY id")
	if err != nil {
		t.Fatalf("ExecuteQuery: %v", err)
	}
	if count != 3 || results[1][1] != "NULL" || results[2][1] != "n2" {
		t.Fatalf("ExecuteQuery: unexpected result %v", results)
	}

	if _, err := db.ExecuteDMLStatement("INSERT INTO " + database + ".logs VALUES ('hello')"); err != nil {
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
	if err := db.ExecutePendingChanges(changes); err != nil {
		t.Fatalf("ExecutePendingChanges: %v", err)
	}

	results, _, err = db.ExecuteQuery("SELECT id, name, note IS NULL AS note_is_null, score FROM " + database + ".events ORDER BY id")
	if err != nil {
		t.Fatalf("ExecuteQuery after changes: %v", err)
	}
	expected := [][]string{
		{"id", "name", "note_is_null", "score"},
		{"1", "alpha'updated", "1", "1.5"},
		{"3", "", "0", "1.5"},
		{"4", "delta", "1", "1.5"},
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
	if _, err := db.Connection.ExecContext(clickHouseMutationContext(), preview); err != nil {
		t.Fatalf("executing preview %q: %v", preview, err)
	}

	if err := db.UpdateRecord(database, "events", "name", "via UpdateRecord", "id", "4"); err != nil {
		t.Fatalf("UpdateRecord: %v", err)
	}
	if err := db.DeleteRecord(database, "events", "id", "1"); err != nil {
		t.Fatalf("DeleteRecord: %v", err)
	}

	results, _, err = db.ExecuteQuery("SELECT id, name FROM " + database + ".events ORDER BY id")
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

	// Mutations are not supported on the TinyLog engine; the server error must
	// be surfaced to the user.
	err = db.ExecutePendingChanges([]models.DBDMLChange{{
		Type:           models.DMLDeleteType,
		Database:       database,
		Table:          "logs",
		PrimaryKeyInfo: []models.PrimaryKeyInfo{{Name: "msg", Value: "hello"}},
	}})
	if err == nil || !strings.Contains(err.Error(), "TinyLog") {
		t.Fatalf("expected mutation error for TinyLog table, got %v", err)
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
