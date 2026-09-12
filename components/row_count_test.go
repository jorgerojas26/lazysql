package components

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/commands"
	"github.com/jorgerojas26/lazysql/drivers"
)

func TestExactCountKeyBinding(t *testing.T) {
	event := tcell.NewEventKey(tcell.KeyRune, '#', tcell.ModNone)
	if command := app.Keymaps.Group(app.TableGroup).Resolve(event); command != commands.ExactCount {
		t.Fatalf("# resolves to %v, want ExactCount", command)
	}
}

func TestPaginationCountStatesAndLookahead(t *testing.T) {
	pagination := NewPagination()
	pagination.SetLimit(2)
	pagination.SetPageInfo(2, true)

	if pagination.GetIsLastPage() {
		t.Fatal("lookahead row should keep the page open")
	}
	if !pagination.GetHasNextPage() {
		t.Fatal("expected next page from lookahead")
	}
	if !strings.Contains(pagination.GetText(), "1-2+") {
		t.Fatalf("unknown total text = %q, want plus form", pagination.GetText())
	}

	pagination.SetOffset(2)
	pagination.SetPageInfo(1, false)
	if pagination.GetTotalRecords() != 3 {
		t.Fatalf("inferred total = %d, want 3", pagination.GetTotalRecords())
	}
	if text := pagination.GetText(); text != "3-3 of 3 rows" {
		t.Fatalf("inferred total text = %q, want exact form", text)
	}

	pagination.SetOffset(0)
	pagination.SetPageInfo(2, true)
	pagination.SetEstimatedTotal(4_300_000)
	text := pagination.GetText()
	if !strings.Contains(text, "~4.3M") || !strings.Contains(text, "[# exact]") {
		t.Fatalf("estimated text = %q, want approximate total and exact hint", text)
	}

	pagination.SetCounting(true)
	if !strings.Contains(pagination.GetText(), "[# cancel]") {
		t.Fatalf("counting text = %q, want cancel hint", pagination.GetText())
	}
	pagination.SetCountError(errors.New("permission denied"))
	if !strings.Contains(pagination.GetText(), "Count failed: permission denied") {
		t.Fatalf("failed text = %q, want non-modal failure", pagination.GetText())
	}
}

type rowCountMock struct {
	schemaProgrammingMock

	mu              sync.Mutex
	estimate        *int64
	estimateErr     error
	exact           int64
	exactErr        error
	estimateCalls   int
	exactCalls      int
	exactStarted    chan struct{}
	exactCanceled   chan struct{}
	exactStartOnce  sync.Once
	exactCancelOnce sync.Once
	waitForContext  bool
}

func (m *rowCountMock) GetEstimatedRowCount(context.Context, string, string) (*int64, error) {
	m.mu.Lock()
	m.estimateCalls++
	estimate, err := m.estimate, m.estimateErr
	m.mu.Unlock()
	return estimate, err
}

func (m *rowCountMock) GetExactRowCount(ctx context.Context, _, _, _ string) (int64, error) {
	m.mu.Lock()
	m.exactCalls++
	exact, err, wait := m.exact, m.exactErr, m.waitForContext
	m.mu.Unlock()
	if m.exactStarted != nil {
		m.exactStartOnce.Do(func() { close(m.exactStarted) })
	}
	if wait {
		<-ctx.Done()
		if m.exactCanceled != nil {
			m.exactCancelOnce.Do(func() { close(m.exactCanceled) })
		}
		return 0, ctx.Err()
	}
	return exact, err
}

func (m *rowCountMock) counts() (estimate, exact int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.estimateCalls, m.exactCalls
}

func newRowCountTestTable(driver drivers.Driver) *ResultsTable {
	return &ResultsTable{
		state: &ResultsTableState{
			databaseName: "database",
			tableName:    "orders",
		},
		Pagination: NewPagination(),
		Menu:       NewResultsTableMenu(),
		DBDriver:   driver,
	}
}

func useSynchronousCountUpdates(t *testing.T) {
	t.Helper()
	oldApplication := app.App.Application
	app.App.Application = nil
	t.Cleanup(func() { app.App.Application = oldApplication })
}

func setCountConfig(t *testing.T, threshold, timeoutMS int) {
	t.Helper()
	config := app.App.Config()
	oldThreshold, oldTimeout := config.ExactCountThreshold, config.ExactCountTimeoutMS
	config.ExactCountThreshold = threshold
	config.ExactCountTimeoutMS = timeoutMS
	t.Cleanup(func() {
		config.ExactCountThreshold = oldThreshold
		config.ExactCountTimeoutMS = oldTimeout
	})
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatal("timed out waiting for row-count operation")
}

func TestAutomaticExactCountUsesEstimateThreshold(t *testing.T) {
	useSynchronousCountUpdates(t)
	setCountConfig(t, 50_000, 500)

	estimate := int64(10)
	driver := &rowCountMock{estimate: &estimate, exact: 7}
	table := newRowCountTestTable(driver)
	table.Pagination.SetLimit(2)
	table.Pagination.SetPageInfo(2, true)
	key := rowCountKey{database: "database", table: "orders"}
	table.prepareRowCountIdentity(key)
	table.startAutomaticRowCount(key)

	waitFor(t, func() bool {
		_, exactCalls := driver.counts()
		return exactCalls == 1
	})
	waitFor(t, func() bool {
		count, ok := table.Pagination.GetExactTotal()
		return ok && count == 7
	})
}

func TestAutomaticCountEstimateAndFilteredPath(t *testing.T) {
	useSynchronousCountUpdates(t)
	setCountConfig(t, 50, 500)

	largeEstimate := int64(100)
	driver := &rowCountMock{estimate: &largeEstimate, exact: 9}
	table := newRowCountTestTable(driver)
	table.Pagination.SetPageInfo(2, true)
	key := rowCountKey{database: "database", table: "orders"}
	table.prepareRowCountIdentity(key)
	table.startAutomaticRowCount(key)

	waitFor(t, func() bool {
		estimateCalls, exactCalls := driver.counts()
		return estimateCalls == 1 && exactCalls == 0
	})
	if _, ok := table.Pagination.GetEstimatedTotal(); !ok {
		t.Fatal("expected an available estimate")
	}

	filteredDriver := &rowCountMock{estimate: &largeEstimate, exact: 4}
	filteredTable := newRowCountTestTable(filteredDriver)
	filteredTable.Pagination.SetPageInfo(2, true)
	filteredKey := rowCountKey{database: "database", table: "orders", where: "WHERE status = 'open'"}
	filteredTable.prepareRowCountIdentity(filteredKey)
	filteredTable.startAutomaticRowCount(filteredKey)
	waitFor(t, func() bool {
		estimateCalls, exactCalls := filteredDriver.counts()
		return estimateCalls == 0 && exactCalls == 1
	})
}

func TestAutomaticCountTimeoutCancelsDriverWithoutTouchingRecords(t *testing.T) {
	useSynchronousCountUpdates(t)
	setCountConfig(t, 50_000, 20)

	estimate := int64(1)
	driver := &rowCountMock{
		estimate:       &estimate,
		exactStarted:   make(chan struct{}),
		exactCanceled:  make(chan struct{}),
		waitForContext: true,
	}
	table := newRowCountTestTable(driver)
	table.Pagination.SetLimit(2)
	table.Pagination.SetPageInfo(2, true)
	key := rowCountKey{database: "database", table: "orders"}
	table.prepareRowCountIdentity(key)
	table.startAutomaticRowCount(key)

	select {
	case <-driver.exactStarted:
	case <-time.After(time.Second):
		t.Fatal("automatic exact count did not start")
	}
	select {
	case <-driver.exactCanceled:
	case <-time.After(time.Second):
		t.Fatal("automatic timeout did not cancel the driver context")
	}
	if table.Pagination.GetIsLastPage() {
		t.Fatal("count timeout incorrectly changed pagination")
	}
	if table.Pagination.HasExactTotal() {
		t.Fatal("timed-out count should not become exact")
	}
}

func TestManualExactCountIgnoresAutomaticTimeoutAndTogglesCancellation(t *testing.T) {
	useSynchronousCountUpdates(t)
	setCountConfig(t, 50_000, 1)

	driver := &rowCountMock{
		exactStarted:   make(chan struct{}),
		exactCanceled:  make(chan struct{}),
		waitForContext: true,
	}
	table := newRowCountTestTable(driver)
	table.Pagination.SetPageInfo(2, true)
	table.ToggleExactCount()

	select {
	case <-driver.exactStarted:
	case <-time.After(time.Second):
		t.Fatal("manual exact count did not start")
	}
	time.Sleep(20 * time.Millisecond)
	select {
	case <-driver.exactCanceled:
		t.Fatal("manual count used the automatic timeout")
	default:
	}
	if !table.IsExactCountActive() {
		t.Fatal("manual count should be active")
	}

	table.ToggleExactCount()
	select {
	case <-driver.exactCanceled:
	case <-time.After(time.Second):
		t.Fatal("second # did not cancel manual count")
	}
	if table.Pagination.GetCountState() == CountFailed {
		t.Fatal("manual cancellation should not report a failure")
	}
}

func TestCountIdentityInvalidatesOnFilterChange(t *testing.T) {
	useSynchronousCountUpdates(t)
	setCountConfig(t, 50_000, 500)

	driver := &rowCountMock{exactStarted: make(chan struct{}), exactCanceled: make(chan struct{}), waitForContext: true}
	table := newRowCountTestTable(driver)
	table.Pagination.SetPageInfo(2, true)
	oldKey := rowCountKey{database: "database", table: "orders"}
	table.prepareRowCountIdentity(oldKey)
	table.startAutomaticRowCount(oldKey)
	select {
	case <-driver.exactStarted:
	case <-time.After(time.Second):
		t.Fatal("exact count did not start")
	}

	newKey := rowCountKey{database: "database", table: "orders", where: "WHERE id > 10"}
	table.prepareRowCountIdentity(newKey)
	select {
	case <-driver.exactCanceled:
	case <-time.After(time.Second):
		t.Fatal("filter change did not cancel the stale count")
	}
	if table.Pagination.HasExactTotal() {
		t.Fatal("filter change retained a stale exact total")
	}
}

func TestAutomaticCountUsesExactFallbackWhenEstimateUnavailable(t *testing.T) {
	useSynchronousCountUpdates(t)
	setCountConfig(t, 50_000, 500)

	driver := &rowCountMock{exact: 5}
	table := newRowCountTestTable(driver)
	table.Pagination.SetPageInfo(2, true)
	key := rowCountKey{database: "database", table: "orders"}
	table.prepareRowCountIdentity(key)
	table.startAutomaticRowCount(key)

	waitFor(t, func() bool {
		_, exactCalls := driver.counts()
		return exactCalls == 1
	})
	count, ok := table.Pagination.GetExactTotal()
	if !ok || count != 5 {
		t.Fatalf("fallback exact total = %d, %t, want 5", count, ok)
	}
}

func TestZeroAutomaticTimeoutStillAllowsEstimateButNotExactCount(t *testing.T) {
	useSynchronousCountUpdates(t)
	setCountConfig(t, 50, 0)

	estimate := int64(10)
	driver := &rowCountMock{estimate: &estimate, exact: 5}
	table := newRowCountTestTable(driver)
	table.Pagination.SetPageInfo(2, true)
	key := rowCountKey{database: "database", table: "orders"}
	table.prepareRowCountIdentity(key)
	table.startAutomaticRowCount(key)

	waitFor(t, func() bool {
		estimateCalls, _ := driver.counts()
		return estimateCalls == 1
	})
	_, exactCalls := driver.counts()
	if exactCalls != 0 {
		t.Fatalf("exact count calls = %d, want 0 when timeout is disabled", exactCalls)
	}
	if _, ok := table.Pagination.GetEstimatedTotal(); !ok {
		t.Fatal("expected estimate with automatic exact-count timeout disabled")
	}
}

func TestManualCountFailureIsRetryable(t *testing.T) {
	useSynchronousCountUpdates(t)
	setCountConfig(t, 50_000, 200)

	driver := &rowCountMock{exact: 8, exactErr: errors.New("permission denied")}
	table := newRowCountTestTable(driver)
	table.Pagination.SetPageInfo(2, true)
	table.ToggleExactCount()
	waitFor(t, func() bool { return table.Pagination.GetCountState() == CountFailed })
	if !strings.Contains(table.Pagination.GetText(), "[# retry]") {
		t.Fatalf("retry text = %q, want retry hint", table.Pagination.GetText())
	}

	driver.mu.Lock()
	driver.exactErr = nil
	driver.mu.Unlock()
	table.ToggleExactCount()
	waitFor(t, func() bool {
		count, ok := table.Pagination.GetExactTotal()
		return ok && count == 8
	})
}
