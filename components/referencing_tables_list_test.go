package components

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gdamore/tcell/v2"

	"github.com/jorgerojas26/lazysql/drivers"
)

var referencingTablesHeader = drivers.ReferencingTablesHeader

func TestBuildReferencingEntries(t *testing.T) {
	testCases := []struct {
		name     string
		rows     [][]string
		expected []referencingTableEntry
	}{
		{
			name:     "no rows",
			rows:     [][]string{referencingTablesHeader},
			expected: nil,
		},
		{
			name: "single column foreign keys",
			rows: [][]string{
				referencingTablesHeader,
				{"LookupList_organizationId_fkey", "public", "LookupList", "organizationId", "id"},
				{"Role_organizationId_fkey", "public", "Role", "organizationId", "id"},
			},
			expected: []referencingTableEntry{
				{Schema: "public", Table: "LookupList", Column: "organizationId", ReferencedColumn: "id"},
				{Schema: "public", Table: "Role", Column: "organizationId", ReferencedColumn: "id"},
			},
		},
		{
			name: "composite foreign keys are skipped",
			rows: [][]string{
				referencingTablesHeader,
				{"composite_fkey", "public", "Membership", "organizationId", "id"},
				{"composite_fkey", "public", "Membership", "tenantId", "tenantId"},
				{"Role_organizationId_fkey", "public", "Role", "organizationId", "id"},
			},
			expected: []referencingTableEntry{
				{Schema: "public", Table: "Role", Column: "organizationId", ReferencedColumn: "id"},
			},
		},
		{
			name: "same constraint name in different tables is not composite",
			rows: [][]string{
				referencingTablesHeader,
				{"fk_org", "public", "Role", "organizationId", "id"},
				{"fk_org", "public", "Category", "organizationId", "id"},
			},
			expected: []referencingTableEntry{
				{Schema: "public", Table: "Role", Column: "organizationId", ReferencedColumn: "id"},
				{Schema: "public", Table: "Category", Column: "organizationId", ReferencedColumn: "id"},
			},
		},
		{
			name: "incomplete rows are skipped",
			rows: [][]string{
				referencingTablesHeader,
				{"short_row", "public", "Role"},
				{"missing_column", "public", "Role", "", "id"},
				{"missing_referenced_column", "public", "Role", "organizationId", ""},
				{"ok", "public", "Role", "organizationId", "id"},
			},
			expected: []referencingTableEntry{
				{Schema: "public", Table: "Role", Column: "organizationId", ReferencedColumn: "id"},
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			entries := buildReferencingEntries(tc.rows)

			if !reflect.DeepEqual(entries, tc.expected) {
				t.Errorf("buildReferencingEntries returned %v, expected %v", entries, tc.expected)
			}
		})
	}
}

func TestReferencingTableEntry_QualifiedTable(t *testing.T) {
	entry := referencingTableEntry{Schema: "public", Table: "LookupList"}

	if got := entry.QualifiedTable(true); got != "public.LookupList" {
		t.Errorf("QualifiedTable(true) returned %q, expected %q", got, "public.LookupList")
	}

	if got := entry.QualifiedTable(false); got != "LookupList" {
		t.Errorf("QualifiedTable(false) returned %q, expected %q", got, "LookupList")
	}

	schemaless := referencingTableEntry{Table: "lookup_list"}

	if got := schemaless.QualifiedTable(true); got != "lookup_list" {
		t.Errorf("QualifiedTable(true) with no schema returned %q, expected %q", got, "lookup_list")
	}
}

func TestNewReferencingTablesListUsesCompactColumns(t *testing.T) {
	entries := []referencingTableEntry{
		{Schema: "public", Table: "orders", Column: "customer_id", ReferencedColumn: "id"},
		{Schema: "audit", Table: "events", Column: "actor_id", ReferencedColumn: "id"},
	}

	picker := NewReferencingTablesList(entries, true, func(int) {}, func() {})
	table := picker.GetTable()

	if table.GetRowCount() != len(entries)+1 {
		t.Fatalf("expected header plus %d entries, got %d rows", len(entries), table.GetRowCount())
	}
	if got := table.GetCell(0, 0).Text; got != "TABLE" {
		t.Errorf("first header is %q, expected TABLE", got)
	}
	if got := table.GetCell(0, 1).Text; got != "RELATIONSHIP" {
		t.Errorf("second header is %q, expected RELATIONSHIP", got)
	}
	if got := table.GetCell(1, 0).Text; got != "public.orders" {
		t.Errorf("first table is %q, expected public.orders", got)
	}
	if got := table.GetCell(1, 1).Text; got != "customer_id  →  id" {
		t.Errorf("first relationship is %q", got)
	}
}

func TestReferencingTablesListFiltersAllVisibleFields(t *testing.T) {
	entries := []referencingTableEntry{
		{Schema: "public", Table: "orders", Column: "customer_id", ReferencedColumn: "id"},
		{Schema: "audit", Table: "events", Column: "actor_id", ReferencedColumn: "user_id"},
	}
	picker := NewReferencingTablesList(entries, true, func(int) {}, func() {})

	testCases := []struct {
		filter        string
		expectedTable string
		expectedIndex int
	}{
		{filter: "audit.events", expectedTable: "audit.events", expectedIndex: 1},
		{filter: "ACTOR", expectedTable: "audit.events", expectedIndex: 1},
		{filter: "customer", expectedTable: "public.orders", expectedIndex: 0},
		{filter: "user_id", expectedTable: "audit.events", expectedIndex: 1},
	}

	for _, tc := range testCases {
		picker.fillTable(tc.filter)
		if got := picker.Table.GetCell(1, 0).Text; got != tc.expectedTable {
			t.Errorf("filter %q returned %q, expected %q", tc.filter, got, tc.expectedTable)
		}
		if !reflect.DeepEqual(picker.filteredIndices, []int{tc.expectedIndex}) {
			t.Errorf("filter %q mapped to %v, expected [%d]", tc.filter, picker.filteredIndices, tc.expectedIndex)
		}
	}

	picker.fillTable("missing")
	if len(picker.filteredIndices) != 0 || picker.Table.GetCell(1, 0).Text != "No matching referencing tables" {
		t.Fatalf("empty filter result was not rendered correctly")
	}
}

func TestReferencingTablesListNavigationKeys(t *testing.T) {
	picker := NewReferencingTablesList([]referencingTableEntry{{Table: "orders", Column: "customer_id", ReferencedColumn: "id"}}, false, func(int) {}, func() {})
	capture := picker.Table.GetInputCapture()

	if event := capture(tcell.NewEventKey(tcell.KeyCtrlN, 0, tcell.ModCtrl)); event.Key() != tcell.KeyDown {
		t.Errorf("Ctrl+N mapped to %v, expected Down", event.Key())
	}
	if event := capture(tcell.NewEventKey(tcell.KeyCtrlP, 0, tcell.ModCtrl)); event.Key() != tcell.KeyUp {
		t.Errorf("Ctrl+P mapped to %v, expected Up", event.Key())
	}
}

func TestResultsTable_GetReferencingEntriesReturnsLookupError(t *testing.T) {
	table := &ResultsTable{state: &ResultsTableState{referencingTablesError: errors.New("permission denied")}}

	entries, err := table.getReferencingEntries()
	if err == nil || err.Error() != "failed to load referencing tables: permission denied" {
		t.Fatalf("getReferencingEntries returned entries=%v, err=%v", entries, err)
	}
}

func TestUseQualifiedReferencingTables(t *testing.T) {
	testCases := []struct {
		name              string
		driverUsesSchemas bool
		provider          string
		expected          bool
	}{
		{name: "schema aware driver", driverUsesSchemas: true, provider: drivers.DriverPostgres, expected: true},
		{name: "MSSQL reverse navigation", provider: drivers.DriverMSSQL, expected: true},
		{name: "schemaless driver", provider: drivers.DriverSqlite, expected: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			if got := useQualifiedReferencingTables(tc.driverUsesSchemas, tc.provider); got != tc.expected {
				t.Errorf("useQualifiedReferencingTables returned %v, expected %v", got, tc.expected)
			}
		})
	}
}
