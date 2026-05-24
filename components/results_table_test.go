package components

import (
	"testing"
	"time"

	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/models"
)

// TestSetLoadingIsSynchronous verifies that SetLoading does not use
// QueueUpdateDraw or any other blocking mechanism.
//
// Regression test for commit d5b0c4b which wrapped SetLoading in
// App.QueueUpdateDraw(), causing a deadlock when called from the
// main tview event loop goroutine (e.g., via tableInputCapture ->
// SetSortedBy when pressing J/K to sort).
//
// QueueUpdateDraw is blocking — it sends to the app's updates channel
// and waits for the event loop to execute the function. When called
// from the main goroutine, the event loop can't process both the
// current event handler and the queued update, causing a deadlock.
func TestSetLoadingIsSynchronous(t *testing.T) {
	changes := []models.DBDMLChange{}

	loadingModal := tview.NewModal()
	errorModal := tview.NewModal()

	pages := tview.NewPages()
	pages.AddPage(pageNameTable, tview.NewFlex(), true, true)
	pages.AddPage(pageNameTableLoading, loadingModal, false, false)
	pages.AddPage(pageNameTableError, errorModal, false, false)

	table := &ResultsTable{
		Table: tview.NewTable(),
		state: &ResultsTableState{
			records:         [][]string{},
			isLoading:       false,
			listOfDBChanges: &changes,
		},
		Page:    pages,
		Loading: loadingModal,
		Error:   errorModal,
	}

	// Verify initial state
	if table.GetIsLoading() {
		t.Error("Expected isLoading to be false initially")
	}

	// SetLoading(true) must return immediately (not block)
	done := make(chan struct{}, 1)
	go func() {
		table.SetLoading(true)
		done <- struct{}{}
	}()

	select {
	case <-done:
		// Success — SetLoading returned synchronously
	case <-time.After(3 * time.Second):
		t.Fatal("SetLoading(true) did not return — likely deadlock from QueueUpdateDraw")
	}

	if !table.GetIsLoading() {
		t.Error("Expected isLoading to be true after SetLoading(true)")
	}

	// SetLoading(false) must also return immediately
	go func() {
		table.SetLoading(false)
		done <- struct{}{}
	}()

	select {
	case <-done:
		// Success — SetLoading returned synchronously
	case <-time.After(3 * time.Second):
		t.Fatal("SetLoading(false) did not return — likely deadlock from QueueUpdateDraw")
	}

	if table.GetIsLoading() {
		t.Error("Expected isLoading to be false after SetLoading(false)")
	}
}
