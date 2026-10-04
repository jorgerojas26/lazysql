package components

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jorgerojas26/lazysql/drivers"
)

type autocompleteSchemaDriver struct {
	schemaProgrammingMock
	useSchemas bool
	tables     map[string][]string
	columns    map[string][][]string
	bulk       map[string][][]string

	tableCalls  atomic.Int32
	columnCalls atomic.Int32
	bulkCalls   atomic.Int32
	mu          sync.Mutex
	bulkNames   []string
}

func (driver *autocompleteSchemaDriver) UseSchemas() bool {
	return driver.useSchemas
}

func (driver *autocompleteSchemaDriver) GetTables(context.Context, string) (map[string][]string, error) {
	driver.tableCalls.Add(1)
	return copySchemaTables(driver.tables), nil
}

func (driver *autocompleteSchemaDriver) GetTableColumns(_ context.Context, _, table string) ([][]string, error) {
	driver.columnCalls.Add(1)
	if columns, ok := driver.columns[table]; ok {
		return columns, nil
	}
	return [][]string{{"column_name"}}, nil
}

func (driver *autocompleteSchemaDriver) GetTableColumnsBulk(_ context.Context, _ string, tables []string) (map[string][][]string, error) {
	driver.bulkCalls.Add(1)
	driver.mu.Lock()
	driver.bulkNames = append([]string(nil), tables...)
	driver.mu.Unlock()
	return driver.bulk, nil
}

func (driver *autocompleteSchemaDriver) requestedBulkTables() []string {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	return append([]string(nil), driver.bulkNames...)
}

func autocompleteColumns(name string) [][]string {
	return [][]string{{"column_name"}, {name + "_id"}, {name + "_name"}}
}

func TestSchemaLoaderSmallSchemaUsesOneBulkRequest(t *testing.T) {
	driver := &autocompleteSchemaDriver{
		tables: map[string][]string{"db": {"orders", "users"}},
		bulk: map[string][][]string{
			"orders": autocompleteColumns("order"),
			"users":  autocompleteColumns("user"),
		},
	}
	loader := newSchemaLoader(driver, newMetadataCache())
	tables := loader.visibleTables("db", driver.tables, nil)
	published := make(map[string][]string)

	loader.preloadEditorColumns(context.Background(), "db", tables, 200, func(table editorSchemaTable, columns []string) {
		published[table.bareName] = columns
	})

	if got := driver.bulkCalls.Load(); got != 1 {
		t.Fatalf("bulk calls = %d, want 1", got)
	}
	if got := driver.columnCalls.Load(); got != 0 {
		t.Fatalf("per-table column calls = %d, want 0", got)
	}
	if got := driver.requestedBulkTables(); len(got) != 2 || got[0] != "orders" || got[1] != "users" {
		t.Fatalf("bulk tables = %v, want sorted orders/users", got)
	}
	if len(published["users"]) != 2 || published["users"][0] != "user_id" {
		t.Fatalf("published users columns = %v", published["users"])
	}

	for _, table := range tables {
		key := newMetadataKey("db", table.qualifiedName, MetadataColumns)
		status, _, err := loader.cache.result(key)
		if err != nil || status != MetadataReady {
			t.Fatalf("cached %s result = status %v, err %v; want ready", table.qualifiedName, status, err)
		}
	}
}

func TestSchemaLoaderLargeSchemaIsLazy(t *testing.T) {
	driver := &autocompleteSchemaDriver{bulk: map[string][][]string{}}
	loader := newSchemaLoader(driver, newMetadataCache())
	tables := make([]editorSchemaTable, 201)
	for i := range tables {
		tables[i] = editorSchemaTable{bareName: "table", qualifiedName: "table"}
	}

	loader.preloadEditorColumns(context.Background(), "db", tables, 200, nil)
	loader.preloadEditorColumns(context.Background(), "db", tables[:1], 0, nil)

	if got := driver.bulkCalls.Load(); got != 0 {
		t.Fatalf("bulk calls = %d, want 0 for large/zero thresholds", got)
	}
	if got := driver.columnCalls.Load(); got != 0 {
		t.Fatalf("per-table column calls = %d, want 0 for lazy preload", got)
	}
}

func TestSchemaLoaderFiltersHiddenSchemasBeforeBulkLoad(t *testing.T) {
	driver := &autocompleteSchemaDriver{
		useSchemas: true,
		tables: map[string][]string{
			"public":  {"users"},
			"private": {"secrets"},
		},
		bulk: map[string][][]string{
			"public.users": autocompleteColumns("user"),
		},
	}
	loader := newSchemaLoader(driver, newMetadataCache())
	visible := loader.visibleTables("db", driver.tables, []string{"public"})
	if len(visible) != 1 || visible[0].qualifiedName != "public.users" {
		t.Fatalf("visible tables = %+v, want only public.users", visible)
	}

	loader.preloadEditorColumns(context.Background(), "db", visible, 200, nil)
	if got := driver.bulkCalls.Load(); got != 1 {
		t.Fatalf("bulk calls = %d, want 1", got)
	}
	if got := driver.requestedBulkTables(); len(got) != 1 || got[0] != "public.users" {
		t.Fatalf("bulk tables = %v, want only public.users", got)
	}
}

func TestSchemaLoaderMSSQLFiltersHiddenSchemasAndKeepsDuplicateIdentity(t *testing.T) {
	loader := newSchemaLoader(&drivers.MSSQL{}, newMetadataCache())
	tables := map[string][]string{
		"audit":   {"users"},
		"dbo":     {"users"},
		"private": {"secrets"},
	}

	visible := loader.visibleTables("test_db", tables, []string{"audit", "dbo"})
	if len(visible) != 2 {
		t.Fatalf("visible tables = %+v, want two configured schemas", visible)
	}
	if visible[0].qualifiedName != "audit.users" || visible[1].qualifiedName != "dbo.users" {
		t.Fatalf("visible table identities = %+v, want audit.users and dbo.users", visible)
	}
	for _, table := range visible {
		if table.qualifiedName == "private.secrets" {
			t.Fatal("hidden schema survived schema filtering")
		}
	}
}

func TestBulkColumnResultDoesNotMergeQualifiedSchemas(t *testing.T) {
	if _, ok := bulkColumnResult(map[string][][]string{"users": autocompleteColumns("bare")}, editorSchemaTable{
		bareName:      "users",
		qualifiedName: "audit.users",
	}); ok {
		t.Fatal("qualified table reused a bare result from another schema")
	}
}

func TestResultsTableKeepsDuplicateSchemaHintsQualified(t *testing.T) {
	table := &ResultsTable{}
	tables := []editorSchemaTable{
		{bareName: "users", qualifiedName: "audit.users"},
		{bareName: "users", qualifiedName: "dbo.users"},
	}
	table.setEditorSchemaTables("test_db", tables)

	if _, _, _, ok := table.editorSchemaTableForHint("users"); ok {
		t.Fatal("ambiguous bare table hint resolved to one schema")
	}
	for _, qualified := range []string{"audit.users", "dbo.users"} {
		got, _, _, ok := table.editorSchemaTableForHint(qualified)
		if !ok || got.qualifiedName != qualified {
			t.Fatalf("qualified hint %q resolved to %+v, ok=%v", qualified, got, ok)
		}
	}
	if got := editorTableNames(tables); len(got) != 2 || got[0] != "audit.users" || got[1] != "dbo.users" {
		t.Fatalf("completion names = %v, want qualified duplicate names", got)
	}
}

func TestSchemaLoaderReusesCachedColumnsWithoutBulkCall(t *testing.T) {
	driver := &autocompleteSchemaDriver{
		tables: map[string][]string{"db": {"users"}},
		bulk:   map[string][][]string{},
	}
	cache := newMetadataCache()
	cache.store(newMetadataKey("db", "users", MetadataColumns), autocompleteColumns("cached"), nil)
	loader := newSchemaLoader(driver, cache)
	published := make(chan []string, 1)

	loader.preloadEditorColumns(context.Background(), "db", []editorSchemaTable{{bareName: "users", qualifiedName: "users"}}, 200, func(_ editorSchemaTable, columns []string) {
		published <- columns
	})

	select {
	case columns := <-published:
		if len(columns) != 2 || columns[0] != "cached_id" {
			t.Fatalf("published cached columns = %v", columns)
		}
	default:
		t.Fatal("cached columns were not published")
	}
	if got := driver.bulkCalls.Load(); got != 0 {
		t.Fatalf("bulk calls = %d, want 0", got)
	}
	if got := driver.columnCalls.Load(); got != 0 {
		t.Fatalf("per-table column calls = %d, want 0", got)
	}
}

func TestSchemaLoaderLoadsOneTableOnDemandAndReusesIt(t *testing.T) {
	driver := &autocompleteSchemaDriver{
		columns: map[string][][]string{"users": autocompleteColumns("user")},
	}
	loader := newSchemaLoader(driver, newMetadataCache())

	key, done := loader.requestColumns(context.Background(), "db", "users")
	if done == nil {
		t.Fatal("first on-demand column request did not start")
	}
	<-done
	status, value, err := loader.cache.result(key)
	if err != nil || status != MetadataReady {
		t.Fatalf("on-demand result = status %v, err %v; want ready", status, err)
	}
	if got := editorColumnNames(value); len(got) != 2 || got[0] != "user_id" {
		t.Fatalf("on-demand columns = %v", got)
	}

	if _, second := loader.requestColumns(context.Background(), "db", "users"); second != nil {
		t.Fatal("cached on-demand columns started another request")
	}
	if got := driver.columnCalls.Load(); got != 1 {
		t.Fatalf("per-table column calls = %d, want 1", got)
	}
}

func TestSchemaLoaderCachesTableListForTreeAndAutocomplete(t *testing.T) {
	driver := &autocompleteSchemaDriver{tables: map[string][]string{"db": {"users"}}}
	loader := newSchemaLoader(driver, newMetadataCache())

	if _, err := loader.loadTables(context.Background(), "db"); err != nil {
		t.Fatalf("first loadTables() error = %v", err)
	}
	if _, err := loader.loadTables(context.Background(), "db"); err != nil {
		t.Fatalf("second loadTables() error = %v", err)
	}
	if got := driver.tableCalls.Load(); got != 1 {
		t.Fatalf("table calls = %d, want one cached call", got)
	}
}

func TestSQLAutocompleteRequestsColumnsForUncachedTable(t *testing.T) {
	editor := NewSQLEditor("")
	called := ""
	editor.SetTables([]string{"users"})
	editor.SetColumnCompletionLoader(func(table string) { called = table })
	editor.SetText("users.", true)

	editor.triggerAutocomplete()
	if called != "users" {
		t.Fatalf("column loader called for %q, want users", called)
	}
}

type invalidatingBulkDriver struct {
	autocompleteSchemaDriver
	invalidate func()
}

func (d *invalidatingBulkDriver) GetTableColumnsBulk(ctx context.Context, database string, tables []string) (map[string][][]string, error) {
	result, err := d.autocompleteSchemaDriver.GetTableColumnsBulk(ctx, database, tables)
	d.invalidate() // refresh races a catalog query already in flight
	return result, err
}

func TestSchemaLoaderDoesNotRestoreInvalidatedBulkColumns(t *testing.T) {
	for _, all := range []bool{false, true} {
		cache := newMetadataCache()
		key := newMetadataKey("db", "users", MetadataColumns)
		d := &invalidatingBulkDriver{autocompleteSchemaDriver: autocompleteSchemaDriver{bulk: map[string][][]string{"users": autocompleteColumns("stale")}}}
		d.invalidate = func() {
			if all {
				cache.invalidateAll()
			} else {
				cache.invalidate(key)
			}
		}
		loader := newSchemaLoader(d, cache)
		published := false
		loader.preloadEditorColumns(context.Background(), "db", []editorSchemaTable{{bareName: "users", qualifiedName: "users"}}, 200, func(editorSchemaTable, []string) { published = true })
		state, _, _ := cache.result(key)
		if state != MetadataUnloaded || published {
			t.Fatalf("all=%t stale bulk metadata survived invalidation: state=%v published=%t", all, state, published)
		}
	}
}

type cancelingColumnDriver struct {
	schemaProgrammingMock
	started  chan struct{}
	canceled chan struct{}
}

func (d *cancelingColumnDriver) GetTableColumns(ctx context.Context, _, _ string) ([][]string, error) {
	close(d.started)
	<-ctx.Done()
	close(d.canceled)
	return nil, ctx.Err()
}

func TestSchemaLoaderInvalidationCancelsColumnWork(t *testing.T) {
	driver := &cancelingColumnDriver{started: make(chan struct{}), canceled: make(chan struct{})}
	cache := newMetadataCache()
	loader := newSchemaLoader(driver, cache)
	key, done := loader.requestColumns(context.Background(), "db", "orders")
	select {
	case <-driver.started:
	case <-time.After(time.Second):
		t.Fatal("column load did not start")
	}
	cache.invalidateAll()
	<-done
	select {
	case <-driver.canceled:
	case <-time.After(time.Second):
		t.Fatal("invalidation did not cancel column load")
	}
	if status, _, _ := cache.result(key); status != MetadataUnloaded {
		t.Fatalf("invalidated state = %v", status)
	}
}
