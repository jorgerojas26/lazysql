package components

import (
	"context"
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/models"
)

func newEditorStreamStateTable() *ResultsTable {
	return &ResultsTable{
		Table: tview.NewTable(),
		state: &ResultsTableState{
			records:         [][]string{},
			listOfDBChanges: &[]models.DBDMLChange{},
		},
		Pagination: NewPagination(),
		DBDriver:   &schemaProgrammingMock{},
	}
}

func TestAppendEditorQueryBatchPreservesRowsAndHeader(t *testing.T) {
	table := newEditorStreamStateTable()
	table.appendEditorQueryBatch(drivers.QueryBatch{
		Columns: []string{"id", "name"},
		Rows:    [][]string{{"1", "alice"}},
	})
	table.appendEditorQueryBatch(drivers.QueryBatch{
		Columns: []string{"id", "name"},
		Rows:    [][]string{{"2", "bob"}},
	})

	want := [][]string{{"id", "name"}, {"1", "alice"}, {"2", "bob"}}
	got := table.GetRecords()
	if len(got) != len(want) || got[1][0] != want[1][0] || got[2][1] != want[2][1] {
		t.Fatalf("records = %v, want %v", got, want)
	}
}

func TestCancelActiveQueryPreservesRenderedRows(t *testing.T) {
	table := newEditorStreamStateTable()
	table.state.records = [][]string{{"id"}, {"1"}}
	ctx, generation := table.startLoad()
	run := table.beginEditorQuery(generation)

	if !table.IsQueryActive() {
		t.Fatal("query was not active after beginEditorQuery")
	}
	if !table.CancelActiveQuery() {
		t.Fatal("CancelActiveQuery() did not cancel the active query")
	}
	if ctx.Err() == nil {
		t.Fatal("CancelActiveQuery() did not cancel the query context")
	}
	if table.IsQueryActive() {
		t.Fatal("cancelled query remained active")
	}
	if got := table.GetRecords(); len(got) != 2 || got[1][0] != "1" {
		t.Fatalf("cancelled query discarded rendered rows: %v", got)
	}
	if !strings.Contains(table.GetQueryStatus(), "partial result: query cancelled") {
		t.Fatalf("query status = %q, want partial cancellation label", table.GetQueryStatus())
	}
	if !strings.Contains(table.Pagination.GetText(), "partial result: query cancelled") {
		t.Fatalf("pagination text = %q, want visible cancellation label", table.Pagination.GetText())
	}
	if table.finishEditorQuery(run) {
		t.Fatal("cancelled query should no longer be finishable")
	}
}

func TestEditorEscapeCancellationIsContextual(t *testing.T) {
	editor := NewSQLEditor("")
	called := false
	editor.SetQueryCancelFunc(func() bool {
		called = true
		return true
	})

	editor.InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone), nil)
	if !called {
		t.Fatal("Escape did not invoke the active-query cancellation callback")
	}
	if editor.vimMode != VimModeInsert {
		t.Fatal("consumed Escape changed normal editor mode")
	}

	called = false
	editor.SetQueryCancelFunc(func() bool {
		called = true
		return false
	})
	editor.InputHandler()(tcell.NewEventKey(tcell.KeyEscape, 0, tcell.ModNone), nil)
	if !called {
		t.Fatal("inactive-query callback was not consulted")
	}
	if editor.vimMode != VimModeNormal {
		t.Fatal("Escape without an active query did not retain editor behavior")
	}
}

func TestStreamEditorQueryUsesDriverContract(t *testing.T) {
	// This compile-time assertion documents the UI/driver seam: the contract is
	// independent of tview and can be implemented by a test or third-party
	// driver without importing components.
	var _ drivers.QueryStreamer = (*streamContractProbe)(nil)
}

type streamContractProbe struct{}

func (*streamContractProbe) StreamQuery(context.Context, string, int, func(drivers.QueryBatch) error) (drivers.QueryStreamResult, error) {
	return drivers.QueryStreamResult{}, nil
}
