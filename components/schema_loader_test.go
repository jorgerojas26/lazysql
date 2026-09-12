package components

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
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
