package components

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/models"
)

func TestMetadataCacheDistinguishesEmptyReadyResult(t *testing.T) {
	cache := newMetadataCache()
	key := newMetadataKey("database", "public.users", MetadataColumns)
	var calls atomic.Int32

	done := cache.request(key, func() (any, error) {
		calls.Add(1)
		return [][]string{}, nil
	})
	if done == nil {
		t.Fatal("expected the first metadata request to load")
	}
	<-done

	status, value, err := cache.result(key)
	if err != nil {
		t.Fatalf("metadata request failed: %v", err)
	}
	if status != MetadataReady {
		t.Fatalf("expected ready status, got %v", status)
	}
	rows, ok := value.([][]string)
	if !ok || rows == nil || len(rows) != 0 {
		t.Fatalf("expected a non-nil empty metadata result, got %#v", value)
	}

	if done := cache.request(key, func() (any, error) {
		calls.Add(1)
		return nil, errors.New("cache was not reused")
	}); done != nil {
		t.Fatal("expected a ready metadata result to be served from cache")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected one underlying metadata query, got %d", got)
	}
}

func TestMetadataCacheIsSharedByTablesWithinOneHome(t *testing.T) {
	home := &Home{}
	first := (&ResultsTable{Home: home}).metadataCacheForTable()
	second := (&ResultsTable{Home: home}).metadataCacheForTable()
	if first != second {
		t.Fatal("expected tables in one connection to share metadata cache")
	}

	other := (&ResultsTable{Home: &Home{}}).metadataCacheForTable()
	if first == other {
		t.Fatal("expected separate connections to have separate metadata caches")
	}
}

func TestMetadataCacheReusesReadyResult(t *testing.T) {
	cache := newMetadataCache()
	key := newMetadataKey("database", "orders", MetadataIndexes)
	var calls atomic.Int32

	load := func() (any, error) {
		calls.Add(1)
		return [][]string{{"index_name"}, {"orders_id"}}, nil
	}

	first := cache.request(key, load)
	<-first
	second := cache.request(key, load)
	if second != nil {
		t.Fatal("expected the second request to use the cached result")
	}

	status, value, err := cache.result(key)
	if err != nil || status != MetadataReady {
		t.Fatalf("expected cached ready result, got status=%v err=%v", status, err)
	}
	if rows := value.([][]string); len(rows) != 2 {
		t.Fatalf("expected cached rows, got %#v", value)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected one underlying metadata query, got %d", got)
	}
}

func TestMetadataCacheRecordsFailureAndAllowsRetry(t *testing.T) {
	cache := newMetadataCache()
	key := newMetadataKey("database", "orders", MetadataForeignKeys)
	var calls atomic.Int32

	first := cache.request(key, func() (any, error) {
		calls.Add(1)
		return nil, errors.New("metadata unavailable")
	})
	<-first

	status, value, err := cache.result(key)
	if status != MetadataFailed {
		t.Fatalf("expected failed status, got %v", status)
	}
	if value != nil || err == nil || err.Error() != "metadata unavailable" {
		t.Fatalf("expected failed result, got value=%#v err=%v", value, err)
	}

	second := cache.request(key, func() (any, error) {
		calls.Add(1)
		return [][]string{{"constraint_name"}}, nil
	})
	if second == nil {
		t.Fatal("expected a failed metadata request to be retryable")
	}
	<-second

	status, value, err = cache.result(key)
	if err != nil || status != MetadataReady {
		t.Fatalf("expected retry to become ready, got status=%v err=%v", status, err)
	}
	if value == nil {
		t.Fatal("expected retry result to be retained")
	}
	if got := calls.Load(); got != 2 {
		t.Fatalf("expected one failed query and one retry, got %d", got)
	}
}

func TestMetadataCacheDeduplicatesInFlightRequests(t *testing.T) {
	cache := newMetadataCache()
	key := newMetadataKey("database", "orders", MetadataConstraints)
	var calls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})

	load := func() (any, error) {
		calls.Add(1)
		close(started)
		<-release
		return [][]string{{"constraint_name"}}, nil
	}

	first := cache.request(key, load)
	<-started
	second := cache.request(key, func() (any, error) {
		calls.Add(1)
		return nil, errors.New("duplicate query")
	})
	if second == nil {
		t.Fatal("expected concurrent request to wait for the in-flight query")
	}
	if first != second {
		t.Fatal("expected concurrent requests to share one completion channel")
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("expected one in-flight query, got %d", got)
	}

	close(release)
	<-first
	status, _, err := cache.result(key)
	if err != nil || status != MetadataReady {
		t.Fatalf("expected deduplicated request to succeed, got status=%v err=%v", status, err)
	}
}

func TestStaleMetadataResultCannotUpdateNewerViewButReachesCache(t *testing.T) {
	cache := newMetadataCache()
	key := newMetadataKey("database", "orders", MetadataColumns)
	table := &ResultsTable{
		state: &ResultsTableState{
			databaseName:          "database",
			tableName:             "orders",
			metadataStates:        map[MetadataKind]MetadataState{},
			foreignKeyColumns:     map[string]bool{},
			foreignKeyJumpTargets: map[string]foreignKeyJumpTarget{},
			fkRawCellValues:       map[string]string{},
		},
		metadataCache: cache,
		Pagination:    NewPagination(),
	}

	oldContext, oldGeneration := table.startLoad()
	done := cache.request(key, func() (any, error) {
		return [][]string{{"column_name"}, {"id"}}, nil
	})
	newContext, newGeneration := table.startLoad()
	<-done

	status, value, err := cache.result(key)
	if err != nil || status != MetadataReady {
		t.Fatalf("expected late result in cache, got status=%v err=%v", status, err)
	}
	if applied := table.applyMetadataResult(oldContext, oldGeneration, "database", "orders", MetadataColumns, status, value, err); applied {
		t.Fatal("stale metadata result updated the newer view")
	}
	if table.GetMetadataState(MetadataColumns) != MetadataUnloaded {
		t.Fatal("stale metadata result changed the newer view state")
	}
	if applied := table.applyMetadataResult(newContext, newGeneration, "database", "orders", MetadataColumns, status, value, err); !applied {
		t.Fatal("current metadata result was rejected")
	}
	if table.GetMetadataState(MetadataColumns) != MetadataReady {
		t.Fatal("current metadata result did not update the view state")
	}
	table.CancelLoading()

	if oldContext.Err() == nil {
		t.Fatal("expected starting the newer view to cancel the old context")
	}
}

func TestMetadataCacheResultForUnknownKeyIsUnloaded(t *testing.T) {
	cache := newMetadataCache()
	status, value, err := cache.result(newMetadataKey("database", "orders", MetadataPrimaryKeys))
	if status != MetadataUnloaded || value != nil || err != nil {
		t.Fatalf("expected unloaded empty result, got status=%v value=%#v err=%v", status, value, err)
	}
}

func TestStaleMetadataIdentityCannotUpdateRevisitedTable(t *testing.T) {
	table := &ResultsTable{
		Table: tview.NewTable(),
		state: &ResultsTableState{
			databaseName:   "database",
			tableName:      "orders",
			metadataStates: newMetadataStates(),
		},
		Pagination: NewPagination(),
	}
	identityGeneration := table.metadataIdentityGenerationValue()
	table.SetTableName("customers")
	table.SetTableName("orders")

	if table.applyMetadataResultForIdentity(identityGeneration, "database", "orders", MetadataColumns, MetadataReady, [][]string{{"column_name"}, {"id"}}, nil) {
		t.Fatal("metadata from an earlier visit updated a newer table identity")
	}
	if table.GetMetadataState(MetadataColumns) != MetadataUnloaded {
		t.Fatal("stale metadata changed the revisited table state")
	}
}

type concurrentMetadataMock struct {
	schemaProgrammingMock
	started chan MetadataKind
	release chan struct{}
	wait    sync.WaitGroup
}

func newConcurrentMetadataMock() *concurrentMetadataMock {
	return &concurrentMetadataMock{
		started: make(chan MetadataKind, len(metadataKinds)),
		release: make(chan struct{}),
	}
}

func (m *concurrentMetadataMock) load(kind MetadataKind, value any) (any, error) {
	m.wait.Add(1)
	defer m.wait.Done()
	m.started <- kind
	<-m.release
	return value, nil
}

func (m *concurrentMetadataMock) GetTableColumns(context.Context, string, string) ([][]string, error) {
	value := [][]string{{"column_name"}, {"id"}}
	result, err := m.load(MetadataColumns, value)
	return result.([][]string), err
}

func (m *concurrentMetadataMock) GetConstraints(context.Context, string, string) ([][]string, error) {
	value := [][]string{{"constraint_name"}, {"orders_pk"}}
	result, err := m.load(MetadataConstraints, value)
	return result.([][]string), err
}

func (m *concurrentMetadataMock) GetForeignKeys(context.Context, string, string) ([][]string, error) {
	value := [][]string{{"constraint_name"}, {"orders_fk"}}
	result, err := m.load(MetadataForeignKeys, value)
	return result.([][]string), err
}

func (m *concurrentMetadataMock) GetIndexes(context.Context, string, string) ([][]string, error) {
	value := [][]string{{"index_name"}, {"orders_id"}}
	result, err := m.load(MetadataIndexes, value)
	return result.([][]string), err
}

func (m *concurrentMetadataMock) GetPrimaryKeyColumnNames(context.Context, string, string) ([]string, error) {
	result, err := m.load(MetadataPrimaryKeys, []string{"id"})
	return result.([]string), err
}

func TestRecordsMetadataStartsKindsConcurrently(t *testing.T) {
	driver := newConcurrentMetadataMock()
	table := newRecordsFetchTestTable(driver)
	ctx, generation := table.startLoad()

	go table.loadRecordsMetadata(ctx, generation, "database", "orders")
	seen := make(map[MetadataKind]bool, len(metadataKinds))
	for range metadataKinds {
		select {
		case kind := <-driver.started:
			seen[kind] = true
		case <-time.After(3 * time.Second):
			t.Fatal("metadata requests did not start concurrently")
		}
	}
	if len(seen) != len(metadataKinds) {
		t.Fatalf("expected all metadata kinds to start, got %v", seen)
	}

	table.CancelLoading()
	close(driver.release)
	driver.wait.Wait()
}

func TestFailedMetadataDoesNotClearRecordsOrOtherMetadata(t *testing.T) {
	changes := []models.DBDMLChange{}
	table := &ResultsTable{
		Table: tview.NewTable(),
		state: &ResultsTableState{
			databaseName:    "database",
			tableName:       "orders",
			records:         [][]string{{"id"}, {"1"}},
			listOfDBChanges: &changes,
			metadataStates:  newMetadataStates(),
		},
		Pagination: NewPagination(),
	}
	ctx, generation := table.startLoad()

	columns := [][]string{{"column_name"}, {"id"}}
	if !table.applyMetadataResult(ctx, generation, "database", "orders", MetadataColumns, MetadataReady, columns, nil) {
		t.Fatal("successful metadata was rejected")
	}
	if !table.applyMetadataResult(ctx, generation, "database", "orders", MetadataConstraints, MetadataFailed, nil, errors.New("constraints unavailable")) {
		t.Fatal("failed metadata was rejected")
	}

	if table.GetMetadataState(MetadataColumns) != MetadataReady {
		t.Fatal("unrelated successful metadata was discarded")
	}
	if table.GetMetadataState(MetadataConstraints) != MetadataFailed {
		t.Fatal("failed metadata state was not retained")
	}
	if table.GetMetadataError(MetadataConstraints) == nil {
		t.Fatal("failed metadata error was not retained")
	}
	if len(table.GetRecords()) != 2 || table.GetRecords()[1][0] != "1" {
		t.Fatal("failed metadata removed Records")
	}
	table.CancelLoading()
}

func TestForeignKeyMetadataEnablesJumpWithoutRecordsReload(t *testing.T) {
	changes := []models.DBDMLChange{}
	db := &drivers.Postgres{}
	db.SetProvider(drivers.DriverPostgres)
	table := &ResultsTable{
		Table: tview.NewTable(),
		state: &ResultsTableState{
			databaseName:          "database",
			tableName:             "public.orders",
			records:               [][]string{{"user_id"}, {"7"}},
			columns:               [][]string{{"Field"}, {"user_id"}},
			listOfDBChanges:       &changes,
			foreignKeyColumns:     map[string]bool{},
			foreignKeyJumpTargets: map[string]foreignKeyJumpTarget{},
			fkRawCellValues:       map[string]string{},
			metadataStates:        newMetadataStates(),
		},
		DBDriver:   db,
		Pagination: NewPagination(),
	}
	table.UpdateRows(table.GetRecords())
	ctx, generation := table.startLoad()

	foreignKeys := [][]string{
		{"constraint_name", "column_name", "foreign_table_name", "foreign_column_name"},
		{"orders_user_fk", "user_id", "users", "id"},
	}
	if !table.applyMetadataResult(ctx, generation, "database", "public.orders", MetadataForeignKeys, MetadataReady, foreignKeys, nil) {
		t.Fatal("foreign-key metadata was rejected for the current view")
	}

	if table.GetMetadataState(MetadataForeignKeys) != MetadataReady {
		t.Fatal("foreign-key metadata did not become ready")
	}
	if !table.isForeignKeyColumn("user_id") {
		t.Fatal("foreign-key column was not enabled")
	}
	_, _, attributes := table.GetCell(1, 0).Style.Decompose()
	if attributes&tcell.AttrUnderline == 0 {
		t.Fatal("foreign-key cell was not underlined after metadata arrived")
	}
	if table.GetRecords()[1][0] != "7" {
		t.Fatal("foreign-key enrichment changed Records")
	}
	table.CancelLoading()
}
