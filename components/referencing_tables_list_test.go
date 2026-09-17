package components

import (
	"reflect"
	"testing"

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
