package drivers

import (
	"math/big"
	"testing"
	"time"

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
			expected: "INSERT INTO `db`.`events` (id, name) VALUES (1, 'O\\'Reilly')",
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

func TestClickHouse_toClickHouseMutation(t *testing.T) {
	table := "`db`.`t`"

	testCases := []struct {
		query    string
		expected string
	}{
		{"UPDATE `db`.`t` SET `a` = ? WHERE `id` = ?", "ALTER TABLE `db`.`t` UPDATE `a` = ? WHERE `id` = ?"},
		{"DELETE FROM `db`.`t` WHERE `id` = ?", "ALTER TABLE `db`.`t` DELETE WHERE `id` = ?"},
		{"INSERT INTO `db`.`t` (a) VALUES (?)", "INSERT INTO `db`.`t` (a) VALUES (?)"},
	}

	for _, tc := range testCases {
		if got := toClickHouseMutation(tc.query, table); got != tc.expected {
			t.Fatalf("expected %q, got %q", tc.expected, got)
		}
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
		{"NULL", "NULL"},
		{"DEFAULT", "DEFAULT"},
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
