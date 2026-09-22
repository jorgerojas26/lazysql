package drivers

import (
	"errors"
	"math/big"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"

	"github.com/jorgerojas26/lazysql/models"
)

func TestClickHouse_DMLChangeToQueryString(t *testing.T) {
	db := &ClickHouse{}

	testCases := []struct {
		name     string
		change   models.DBDMLChange
		expected string
	}{
		{
			name: "Insert",
			change: models.DBDMLChange{
				Type:     models.DMLInsertType,
				Database: "db",
				Table:    "events",
				Values: []models.CellValue{
					{Column: "id", Value: 1, Type: models.String},
					{Column: "name", Value: "O'Reilly", Type: models.String},
				},
			},
			expected: "INSERT INTO `db`.`events` (`id`, `name`) VALUES (1, 'O\\'Reilly')",
		},
		{
			name: "Update",
			change: models.DBDMLChange{
				Type:     models.DMLUpdateType,
				Database: "db",
				Table:    "events",
				Values: []models.CellValue{
					{Column: "name", Value: "new", Type: models.String},
					{Column: "note", Value: "NULL", Type: models.Null},
				},
				PrimaryKeyInfo: []models.PrimaryKeyInfo{{Name: "id", Value: "1"}},
			},
			expected: "ALTER TABLE `db`.`events` UPDATE `name` = 'new', `note` = NULL WHERE `id` = '1'",
		},
		{
			name: "Delete",
			change: models.DBDMLChange{
				Type:           models.DMLDeleteType,
				Database:       "db",
				Table:          "events",
				PrimaryKeyInfo: []models.PrimaryKeyInfo{{Name: "id", Value: "1"}, {Name: "day", Value: "2024-01-01"}},
			},
			expected: "ALTER TABLE `db`.`events` DELETE WHERE `id` = '1' AND `day` = '2024-01-01'",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := db.DMLChangeToQueryString(tc.change)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestClickHouse_FormatArgForQueryString(t *testing.T) {
	db := &ClickHouse{}

	testCases := []struct {
		arg      any
		expected string
	}{
		{123, "123"},
		{1.50, "1.5"},
		{"NULL", "'NULL'"},
		{"DEFAULT", "'DEFAULT'"},
		{"O'Reilly", `'O\'Reilly'`},
		{`C:\dir`, `'C:\\dir'`},
		{[]byte("bytes"), "'bytes'"},
	}

	for _, tc := range testCases {
		if got := db.FormatArgForQueryString(tc.arg); got != tc.expected {
			t.Fatalf("FormatArgForQueryString(%v): expected %q, got %q", tc.arg, tc.expected, got)
		}
	}
}

func TestClickHouse_FormatArg(t *testing.T) {
	db := &ClickHouse{}

	if got := db.FormatArg("x", models.Null); got != nil {
		t.Fatalf("expected nil for NULL, got %v", got)
	}
	if got := db.FormatArg("x", models.Empty); got != "" {
		t.Fatalf("expected empty string, got %v", got)
	}
	if got := db.FormatArg(42, models.String); got != "42" {
		t.Fatalf("expected \"42\", got %v", got)
	}
}

func TestClickHouse_clickHouseValueToString(t *testing.T) {
	str := "value"
	var nilStr *string
	ts := time.Date(2024, 1, 2, 3, 4, 5, 123000000, time.UTC)

	testCases := []struct {
		name         string
		value        any
		databaseType string
		expected     string
		isNull       bool
	}{
		{"nil", nil, "String", "", true},
		{"nil pointer", nilStr, "Nullable(String)", "", true},
		{"pointer", &str, "Nullable(String)", "value", false},
		{"uint", uint64(7), "UInt64", "7", false},
		{"date", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), "Date", "2024-01-02", false},
		{"nullable date32", time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), "Nullable(Date32)", "2024-01-02", false},
		{"datetime", time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC), "DateTime", "2024-01-02 03:04:05", false},
		{"datetime64", ts, "DateTime64(3)", "2024-01-02 03:04:05.123", false},
		{"array", []string{"a", "it's"}, "Array(String)", `['a','it\'s']`, false},
		{"empty array", []uint8{}, "Array(UInt8)", "[]", false},
		{"nested array", [][]int64{{1, 2}, {3}}, "Array(Array(Int64))", "[[1,2],[3]]", false},
		{"nullable array", []*string{&str, nil}, "Array(Nullable(String))", "['value',NULL]", false},
		{"tuple", []any{int32(1), "z"}, "Tuple(Int32, String)", "(1,'z')", false},
		{"int128", *big.NewInt(-5), "Int128", "-5", false},
		{"int256 pointer", big.NewInt(9), "Nullable(Int256)", "9", false},
		{"map", map[string]uint32{"b": 2, "a": 1}, "Map(String, UInt32)", "{'a':1,'b':2}", false},
		{"nullable tuple", []any{int32(2), "x"}, "Nullable(Tuple(Int32, String))", "(2,'x')", false},
		{"array of tuples", [][]any{{int32(3), "y"}, {int32(4), "z"}}, "Array(Tuple(Int32, String))", "[(3,'y'),(4,'z')]", false},
		{"array of dates", []time.Time{time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)}, "Array(Date)", "['2024-01-02']", false},
		{"array of UUIDs", []uuid.UUID{uuid.MustParse("61f0c404-5cb3-11e7-907b-a6006ad3dba0")}, "Array(UUID)", "['61f0c404-5cb3-11e7-907b-a6006ad3dba0']", false},
		{"map with tuple values", map[string][]any{"k": {int32(5), "w"}}, "Map(String, Tuple(Int32, String))", "{'k':(5,'w')}", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, isNull := clickHouseValueToString(tc.value, tc.databaseType)
			if got != tc.expected || isNull != tc.isNull {
				t.Fatalf("expected (%q, %v), got (%q, %v)", tc.expected, tc.isNull, got, isNull)
			}
		})
	}
}

func TestClickHouse_FormatReferenceEscapesBackticks(t *testing.T) {
	db := &ClickHouse{}
	if got := db.formatTableName("lazy`db", "t`x"); got != "`lazy``db`.`t``x`" {
		t.Fatalf("unexpected quoted table name: %s", got)
	}
	if got := db.FormatReference("c`x"); got != "`c``x`" {
		t.Fatalf("unexpected quoted column name: %s", got)
	}
}

func TestClickHouse_DMLChangeToQueryStringUsesCompositeLiterals(t *testing.T) {
	connection, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()

	db := &ClickHouse{Connection: connection}
	mock.ExpectQuery(regexp.QuoteMeta("SELECT name, type FROM system.columns WHERE database = ? AND table = ?")).
		WithArgs("db", "events").
		WillReturnRows(sqlmock.NewRows([]string{"name", "type"}).
			AddRow("id", "UInt64").
			AddRow("attrs", "Map(String, UInt32)"))

	query, err := db.DMLChangeToQueryString(models.DBDMLChange{
		Type:     models.DMLUpdateType,
		Database: "db",
		Table:    "events",
		Values:   []models.CellValue{{Column: "attrs", Value: "{'z':2}", Type: models.String}},
		PrimaryKeyInfo: []models.PrimaryKeyInfo{{
			Name:  "id",
			Value: "1",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	expected := "ALTER TABLE `db`.`events` UPDATE `attrs` = CAST(map('z', 2) AS Map(String, UInt32)) WHERE `id` = '1'"
	if query != expected {
		t.Fatalf("expected %q, got %q", expected, query)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClickHouse_CompositeExpression(t *testing.T) {
	testCases := []struct {
		literal    string
		columnType string
		expected   string
	}{
		{"{'z':2}", "Map(String, UInt32)", "map('z', 2)"},
		{"[{'a':1},{'b':2}]", "Array(Map(String, UInt32))", "[map('a', 1), map('b', 2)]"},
		{"({'a':1},['x'])", "Tuple(Map(String, UInt32), Array(String))", "(map('a', 1), ['x'])"},
	}
	for _, tc := range testCases {
		got, err := clickHouseCompositeExpression(tc.literal, tc.columnType)
		if err != nil {
			t.Fatalf("%s: %v", tc.columnType, err)
		}
		if got != tc.expected {
			t.Fatalf("%s: expected %q, got %q", tc.columnType, tc.expected, got)
		}
	}
}

func TestClickHouse_ExecutePendingChangesReportsAppliedPrefix(t *testing.T) {
	connection, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()

	columnsQuery := regexp.QuoteMeta("SELECT name, type FROM system.columns WHERE database = ? AND table = ?")
	for range 2 {
		mock.ExpectQuery(columnsQuery).
			WithArgs("db", "events").
			WillReturnRows(sqlmock.NewRows([]string{"name", "type"}).AddRow("id", "UInt64"))
	}
	mock.ExpectExec(regexp.QuoteMeta("INSERT INTO `db`.`events` (`id`) VALUES (?)")).
		WithArgs("1").
		WillReturnResult(sqlmock.NewResult(0, 1))
	failure := errors.New("mutation failed")
	mock.ExpectExec(regexp.QuoteMeta("ALTER TABLE `db`.`events` DELETE WHERE `id` = ?")).
		WithArgs("2").
		WillReturnError(failure)

	db := &ClickHouse{Connection: connection}
	err = db.ExecutePendingChanges([]models.DBDMLChange{
		{
			Type:     models.DMLInsertType,
			Database: "db",
			Table:    "events",
			Values:   []models.CellValue{{Column: "id", Value: "1", Type: models.String}},
		},
		{
			Type:           models.DMLDeleteType,
			Database:       "db",
			Table:          "events",
			PrimaryKeyInfo: []models.PrimaryKeyInfo{{Name: "id", Value: "2"}},
		},
	})
	var partialErr *PartialExecutionError
	if !errors.As(err, &partialErr) {
		t.Fatalf("expected PartialExecutionError, got %v", err)
	}
	if partialErr.Applied != 1 || !errors.Is(err, failure) {
		t.Fatalf("unexpected partial error: %#v", partialErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestClickHouse_GetForeignKeys(t *testing.T) {
	db := &ClickHouse{}

	fks, err := db.GetForeignKeys("db", "t")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fks) != 1 {
		t.Fatalf("expected only a header row, got %v", fks)
	}
}

func TestClickHouse_SchemasAndProgramming(t *testing.T) {
	db := &ClickHouse{}

	if db.UseSchemas() {
		t.Fatal("ClickHouse should not use schemas")
	}
	if db.SupportsProgramming() {
		t.Fatal("ClickHouse should not support programming")
	}
}
