package components

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/models"
)

type refreshCallDriver struct {
	schemaProgrammingMock

	mu       sync.Mutex
	calls    map[MetadataKind]int
	pageCall int
	where    string
	sort     string
	offset   int
	limit    int
}

func newRefreshCallDriver() *refreshCallDriver {
	return &refreshCallDriver{calls: make(map[MetadataKind]int)}
}

func (driver *refreshCallDriver) record(kind MetadataKind) {
	driver.mu.Lock()
	driver.calls[kind]++
	driver.mu.Unlock()
}

func (driver *refreshCallDriver) count(kind MetadataKind) int {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	return driver.calls[kind]
}

func (driver *refreshCallDriver) GetTableColumns(string, string) ([][]string, error) {
	driver.record(MetadataColumns)
	return [][]string{{"column_name"}, {"id"}}, nil
}

func (driver *refreshCallDriver) GetPrimaryKeyColumnNames(string, string) ([]string, error) {
	driver.record(MetadataPrimaryKeys)
	return []string{"id"}, nil
}

func (driver *refreshCallDriver) GetForeignKeys(context.Context, string, string) ([][]string, error) {
	driver.record(MetadataForeignKeys)
	return [][]string{{"constraint_name"}, {"orders_user_fk"}}, nil
}

type retryForeignKeyDriver struct {
	*refreshCallDriver
	mu   sync.Mutex
	fail bool
}

func (driver *retryForeignKeyDriver) GetForeignKeys(context.Context, string, string) ([][]string, error) {
	driver.record(MetadataForeignKeys)
	driver.mu.Lock()
	fail := driver.fail
	driver.mu.Unlock()
	if fail {
		return nil, errors.New("foreign keys unavailable")
	}
	return [][]string{{"constraint_name"}, {"orders_user_fk"}}, nil
}

func (driver *retryForeignKeyDriver) setFail(fail bool) {
	driver.mu.Lock()
	driver.fail = fail
	driver.mu.Unlock()
}

type pendingForeignKeyDriver struct {
	*refreshCallDriver
	started  chan struct{}
	released chan struct{}
	finished chan struct{}
}

func newPendingForeignKeyDriver() *pendingForeignKeyDriver {
	return &pendingForeignKeyDriver{
		refreshCallDriver: newRefreshCallDriver(),
		started:           make(chan struct{}),
		released:          make(chan struct{}),
		finished:          make(chan struct{}),
	}
}

func (driver *pendingForeignKeyDriver) GetProvider() string {
	return drivers.DriverPostgres
}

func (driver *pendingForeignKeyDriver) GetForeignKeys(context.Context, string, string) ([][]string, error) {
	driver.record(MetadataForeignKeys)
	close(driver.started)
	<-driver.released
	close(driver.finished)
	return [][]string{
		{"constraint_name", "column_name", "foreign_table_name", "foreign_column_name"},
		{"orders_user_fk", "user_id", "users", "id"},
	}, nil
}

type blockingProviderDriver struct {
	*refreshCallDriver
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func newBlockingProviderDriver() *blockingProviderDriver {
	return &blockingProviderDriver{
		refreshCallDriver: newRefreshCallDriver(),
		started:           make(chan struct{}),
		release:           make(chan struct{}),
	}
}

func (driver *blockingProviderDriver) GetProvider() string {
	driver.once.Do(func() { close(driver.started) })
	<-driver.release
	return drivers.DriverPostgres
}

func (driver *blockingProviderDriver) GetForeignKeys(context.Context, string, string) ([][]string, error) {
	return [][]string{
		{"constraint_name", "column_name", "foreign_table_name", "foreign_column_name"},
		{"orders_user_fk", "user_id", "users", "id"},
	}, nil
}

func (driver *refreshCallDriver) GetConstraints(string, string) ([][]string, error) {
	driver.record(MetadataConstraints)
	return [][]string{{"constraint_name"}, {"orders_pk"}}, nil
}

func (driver *refreshCallDriver) GetIndexes(string, string) ([][]string, error) {
	driver.record(MetadataIndexes)
	return [][]string{{"index_name"}, {"orders_id"}}, nil
}

func (driver *refreshCallDriver) GetRecords(_ context.Context, _, _ string, where, sort string, offset, limit int) (drivers.PageResult, error) {
	driver.mu.Lock()
	driver.pageCall++
	driver.where = where
	driver.sort = sort
	driver.offset = offset
	driver.limit = limit
	driver.mu.Unlock()
	return drivers.PageResult{Rows: [][]string{{"id"}, {"1"}}}, nil
}

func (driver *refreshCallDriver) pageArgs() (int, string, string, int, int) {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	return driver.pageCall, driver.where, driver.sort, driver.offset, driver.limit
}

func newRefreshCallTable(driver drivers.Driver) *ResultsTable {
	changes := []models.DBDMLChange{}
	table := &ResultsTable{
		Table: tview.NewTable(),
		state: &ResultsTableState{
			records:               [][]string{{"id"}, {"old"}},
			columns:               [][]string{{"column_name"}, {"id"}},
			constraints:           [][]string{{"constraint_name"}, {"orders_pk"}},
			foreignKeys:           [][]string{{"constraint_name"}, {"orders_user_fk"}},
			indexes:               [][]string{{"index_name"}, {"orders_id"}},
			foreignKeyColumns:     map[string]bool{},
			foreignKeyJumpTargets: map[string]foreignKeyJumpTarget{},
			fkRawCellValues:       map[string]string{},
			markedRows:            map[int]bool{},
			metadataStates:        newMetadataStates(),
			metadataErrors:        map[MetadataKind]error{},
			listOfDBChanges:       &changes,
		},
		Pagination: NewPagination(),
		DBDriver:   driver,
		Menu:       NewResultsTableMenu(),
		Filter:     NewResultsFilter(),
	}
	table.SetDatabaseName("database")
	table.SetTableName("orders")
	return table
}

func primeRefreshMetadata(t *testing.T, table *ResultsTable) {
	t.Helper()
	for _, kind := range metadataKinds {
		_, done := table.requestMetadata("database", "orders", kind)
		if done != nil {
			<-done
		}
	}
}

func waitForRefreshCount(t *testing.T, driver *refreshCallDriver, kind MetadataKind, want int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if driver.count(kind) >= want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s call count %d; got %d", kind, want, driver.count(kind))
}

func waitForForeignKeyMetadata(t *testing.T, table *ResultsTable) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		applied := false
		App.QueueUpdate(func() {
			applied = table.GetMetadataState(MetadataForeignKeys) == MetadataReady && table.isForeignKeyColumn("user_id")
		})
		if applied {
			return
		}
	}
	t.Fatalf("timed out waiting for Foreign Keys metadata to apply")
}

func startRefreshApplication(t *testing.T, table *ResultsTable) func() {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatalf("init simulation screen: %v", err)
	}
	App.SetScreen(screen)

	root := tview.NewPages()
	root.AddPage(pageNameTable, table, true, true)
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = App.Run(root, "")
	}()
	App.QueueUpdate(func() {})

	return func() {
		App.QueueUpdate(func() {
			table.CancelLoading()
		})
		App.Application.Stop()
		<-done
	}
}

func TestRefreshMetadataReloadsOnlyActiveKind(t *testing.T) {
	cases := []struct {
		option int
		kind   MetadataKind
	}{
		{option: 2, kind: MetadataColumns},
		{option: 3, kind: MetadataConstraints},
		{option: 4, kind: MetadataForeignKeys},
		{option: 5, kind: MetadataIndexes},
	}

	for _, testCase := range cases {
		t.Run(string(testCase.kind), func(t *testing.T) {
			driver := newRefreshCallDriver()
			table := newRefreshCallTable(driver)
			primeRefreshMetadata(t, table)
			stopApp := startRefreshApplication(t, table)
			defer stopApp()

			table.Menu.SetSelectedOption(testCase.option)
			table.RefreshActiveSurface()
			waitForRefreshCount(t, driver, testCase.kind, 2)

			for _, kind := range metadataKinds {
				want := 1
				if kind == testCase.kind {
					want = 2
				}
				if got := driver.count(kind); got != want {
					t.Fatalf("%s calls = %d, want %d", kind, got, want)
				}
			}
			if pageCalls, _, _, _, _ := driver.pageArgs(); pageCalls != 0 {
				t.Fatalf("metadata refresh unexpectedly fetched %d Records pages", pageCalls)
			}
		})
	}
}

func TestRefreshPreservesUnrelatedPendingMetadata(t *testing.T) {
	driver := newPendingForeignKeyDriver()
	table := newRefreshCallTable(driver)

	// Seed the active surface so RefreshMetadata performs exactly one reload.
	_, done := table.requestMetadata("database", "orders", MetadataColumns)
	if done != nil {
		<-done
	}

	stopApp := startRefreshApplication(t, table)
	defer stopApp()

	ctx, generation := table.startLoad()
	table.loadMetadataKind(ctx, generation, "database", "orders", MetadataForeignKeys)
	<-driver.started
	if table.GetMetadataState(MetadataForeignKeys) != MetadataLoading {
		t.Fatal("expected Foreign Keys to remain loading before controlled completion")
	}

	table.Menu.SetSelectedOption(2)
	table.RefreshActiveSurface()
	waitForRefreshCount(t, driver.refreshCallDriver, MetadataColumns, 2)
	if got := driver.count(MetadataColumns); got != 2 {
		t.Fatalf("Columns calls = %d, want exactly 2", got)
	}

	close(driver.released)
	<-driver.finished
	waitForForeignKeyMetadata(t, table)
	foreignKeys := table.GetForeignKeys()
	if len(foreignKeys) != 2 || foreignKeys[1][0] != "orders_user_fk" {
		t.Fatalf("unrelated Foreign Keys result was not applied: %v", foreignKeys)
	}
	if !table.isForeignKeyColumn("user_id") {
		t.Fatal("pending Foreign Keys result did not enable Foreign Key Jump")
	}
	if target, ok := table.getForeignKeyJumpTarget("user_id"); !ok || target.ReferencedTable != "users" || target.ReferencedColumn != "id" {
		t.Fatalf("Foreign Key Jump target = %#v, present = %v", target, ok)
	}
	if got := driver.count(MetadataForeignKeys); got != 1 {
		t.Fatalf("Foreign Keys calls = %d, want exactly 1", got)
	}
	if pageCalls, _, _, _, _ := driver.pageArgs(); pageCalls != 0 {
		t.Fatalf("metadata refresh unexpectedly fetched %d Records pages", pageCalls)
	}
}

func TestMetadataApplySerializesTableIdentityChanges(t *testing.T) {
	driver := newBlockingProviderDriver()
	table := newRefreshCallTable(driver)
	identityGeneration := table.metadataIdentityGenerationValue()

	applied := make(chan bool)
	go func() {
		applied <- table.applyMetadataResultForIdentity(identityGeneration, "database", "orders", MetadataForeignKeys, MetadataReady, [][]string{
			{"constraint_name", "column_name", "foreign_table_name", "foreign_column_name"},
			{"orders_user_fk", "user_id", "users", "id"},
		}, nil)
	}()
	<-driver.started

	identityChanged := make(chan struct{})
	go func() {
		table.SetTableName("customers")
		close(identityChanged)
	}()
	select {
	case <-identityChanged:
		t.Fatal("table identity changed while metadata was being applied")
	case <-time.After(50 * time.Millisecond):
	}

	close(driver.release)
	if !<-applied {
		t.Fatal("current metadata result was rejected")
	}
	<-identityChanged
	if table.GetTableName() != "customers" {
		t.Fatal("table identity update did not complete")
	}
}

func TestRecordsRefreshPreservesPendingNonPrimaryKeyMetadata(t *testing.T) {
	driver := newPendingForeignKeyDriver()
	table := newRefreshCallTable(driver)
	stopApp := startRefreshApplication(t, table)
	defer stopApp()

	ctx, generation := table.startLoad()
	table.loadMetadataKind(ctx, generation, "database", "orders", MetadataForeignKeys)
	<-driver.started

	table.RefreshRecords()
	waitForRefreshCount(t, driver.refreshCallDriver, MetadataPrimaryKeys, 1)

	close(driver.released)
	<-driver.finished
	waitForForeignKeyMetadata(t, table)
	foreignKeys := table.GetForeignKeys()
	if len(foreignKeys) != 2 || foreignKeys[1][0] != "orders_user_fk" {
		t.Fatalf("pending Foreign Keys result was not applied: %v", foreignKeys)
	}
	if !table.isForeignKeyColumn("user_id") {
		t.Fatal("pending Foreign Keys result did not enable Foreign Key Jump")
	}
	if got := driver.count(MetadataForeignKeys); got != 1 {
		t.Fatalf("Foreign Keys calls = %d, want exactly 1", got)
	}
	if pageCalls, _, _, _, _ := driver.pageArgs(); pageCalls != 1 {
		t.Fatalf("Records refresh made %d page calls, want exactly 1", pageCalls)
	}
}

func TestRefreshRecordsPreservesQueryAndRetriesOnlyPrimaryKeys(t *testing.T) {
	driver := newRefreshCallDriver()
	table := newRefreshCallTable(driver)
	primeRefreshMetadata(t, table)
	table.state.currentSort = "id DESC"
	table.Filter.SetCurrentFilterUnsafe("WHERE id > 10")
	table.Pagination.SetOffset(30)
	table.Pagination.SetLimit(25)

	stopApp := startRefreshApplication(t, table)
	defer stopApp()

	table.RefreshActiveSurface()
	waitForRefreshCount(t, driver, MetadataPrimaryKeys, 1)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if pageCalls, _, _, _, _ := driver.pageArgs(); pageCalls == 1 {
			break
		}
		time.Sleep(time.Millisecond)
	}

	pageCalls, where, sort, offset, limit := driver.pageArgs()
	if pageCalls != 1 {
		t.Fatalf("Records refresh made %d page calls, want 1", pageCalls)
	}
	if where != "WHERE id > 10" || sort != "id DESC" || offset != 30 || limit != 25 {
		t.Fatalf("Records refresh query identity = where %q sort %q offset %d limit %d", where, sort, offset, limit)
	}
	for _, kind := range []MetadataKind{MetadataColumns, MetadataForeignKeys, MetadataConstraints, MetadataIndexes} {
		if got := driver.count(kind); got != 1 {
			t.Fatalf("Records refresh made %d %s calls, want cached metadata only", got-1, kind)
		}
	}
}

func TestFailedMetadataSurfaceShowsLocalRetryMessage(t *testing.T) {
	driver := newRefreshCallDriver()
	table := newRefreshCallTable(driver)
	table.Menu.SetSelectedOption(4)
	ctx, generation := table.startLoad()

	if !table.applyMetadataResult(ctx, generation, "database", "orders", MetadataForeignKeys, MetadataFailed, nil, errors.New("permission denied")) {
		t.Fatal("failed metadata result was rejected")
	}

	if got := table.GetCell(0, 0).Text; got != "Foreign Keys unavailable — press R to retry" {
		t.Fatalf("failure surface = %q", got)
	}
	if got := table.GetRecords(); len(got) != 2 || got[1][0] != "old" {
		t.Fatalf("failed metadata changed Records: %v", got)
	}
	if table.GetMetadataError(MetadataForeignKeys) == nil {
		t.Fatal("failed metadata error was not retained")
	}
	table.CancelLoading()
}

func TestFailedMetadataRefreshRetriesOnlyThatSurface(t *testing.T) {
	driver := &retryForeignKeyDriver{refreshCallDriver: newRefreshCallDriver()}
	table := newRefreshCallTable(driver)
	primeRefreshMetadata(t, table)
	driver.setFail(true)

	stopApp := startRefreshApplication(t, table)
	defer stopApp()

	table.Menu.SetSelectedOption(4)
	table.RefreshActiveSurface()
	waitForRefreshCount(t, driver.refreshCallDriver, MetadataForeignKeys, 2)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && table.GetMetadataState(MetadataForeignKeys) != MetadataFailed {
		time.Sleep(time.Millisecond)
	}
	if table.GetMetadataState(MetadataForeignKeys) != MetadataFailed {
		t.Fatal("failed metadata refresh did not expose a failed state")
	}
	if got := table.GetCell(0, 0).Text; got != "Foreign Keys unavailable — press R to retry" {
		t.Fatalf("failure surface = %q", got)
	}

	driver.setFail(false)
	table.RefreshActiveSurface()
	waitForRefreshCount(t, driver.refreshCallDriver, MetadataForeignKeys, 3)
	deadline = time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) && table.GetMetadataState(MetadataForeignKeys) != MetadataReady {
		time.Sleep(time.Millisecond)
	}
	if table.GetMetadataState(MetadataForeignKeys) != MetadataReady {
		t.Fatal("failed metadata surface was not retried")
	}
	if got := table.GetCell(1, 0).Text; got != "orders_user_fk" {
		t.Fatalf("retried metadata was not rendered: %q", got)
	}
}
