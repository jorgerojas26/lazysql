package components

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers/logger"
)

// editorSchemaTable is the driver-facing name and the autocomplete-facing
// name for one visible table. Schema-aware drivers need the qualified name for
// catalog lookups, while the editor displays bare names when they are unique.
type editorSchemaTable struct {
	bareName      string
	qualifiedName string
}

// schemaLoader owns schema-list and column loading for one connection. Its
// cache is shared with ResultsTable metadata so the tree, Records, and SQL
// autocomplete can reuse the same table/column results.
type schemaLoader struct {
	driver drivers.Driver
	cache  *metadataCache
}

func newSchemaLoader(driver drivers.Driver, cache *metadataCache) *schemaLoader {
	if cache == nil {
		cache = newMetadataCache()
	}
	return &schemaLoader{driver: driver, cache: cache}
}

// EditorSchemaLoadResult contains the observable results of loading the SQL
// editor's table and column catalog.
type EditorSchemaLoadResult struct {
	TableNames  []string
	ColumnNames map[string][]string
}

// LoadEditorSchema runs the production schema-loader path without requiring a
// ResultsTable or a UI event loop. The table callback runs after the table list
// is ready and before column enrichment starts, which lets callers measure the
// progressive first useful result separately from background completion.
//
// Small schemas use the same bulk-column preload as the editor. Larger schemas
// model the first on-demand column completion so the threshold remains an
// observable production behavior rather than a scenario-only counter.
func LoadEditorSchema(
	ctx context.Context,
	driver drivers.Driver,
	database string,
	threshold int,
	onTablesReady func(int),
) (EditorSchemaLoadResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if driver == nil {
		return EditorSchemaLoadResult{}, errors.New("schema loader driver is nil")
	}
	if database == "" {
		return EditorSchemaLoadResult{}, errors.New("database name is required")
	}

	loader := newSchemaLoader(driver, newMetadataCache())
	tablesMap, err := loader.loadTables(ctx, database)
	if err != nil {
		return EditorSchemaLoadResult{}, err
	}

	tableList := loader.visibleTables(database, tablesMap, nil)
	result := EditorSchemaLoadResult{
		TableNames:  editorTableNames(tableList),
		ColumnNames: make(map[string][]string),
	}
	if onTablesReady != nil {
		onTablesReady(len(result.TableNames))
	}

	publish := func(table editorSchemaTable, columns []string) {
		result.ColumnNames[table.qualifiedName] = append([]string(nil), columns...)
	}
	loader.preloadEditorColumns(ctx, database, tableList, threshold, publish)

	if len(tableList) == 0 || (threshold > 0 && len(tableList) <= threshold) {
		return result, nil
	}

	// A large schema is lazy in the normal editor. Request one table exactly as
	// the first table.column completion would, so the benchmark observes that
	// production request without recreating the N+1 preload path.
	key, done := loader.requestColumns(ctx, database, tableList[0].qualifiedName)
	if done != nil {
		<-done
	}
	status, value, err := loader.cache.result(key)
	if err != nil {
		return EditorSchemaLoadResult{}, err
	}
	if status != MetadataReady {
		return EditorSchemaLoadResult{}, errors.New("schema column result is not ready")
	}
	result.ColumnNames[tableList[0].qualifiedName] = editorColumnNames(value)
	return result, nil
}

func (loader *schemaLoader) requestTables(ctx context.Context, database string) (metadataKey, <-chan struct{}) {
	key := newMetadataKey(database, "", MetadataTables)
	if loader == nil || loader.driver == nil {
		return key, nil
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	done := loader.cache.requestWithContext(ctx, key, cancel, func() (any, error) {
		started := time.Now()
		tables, err := loader.driver.GetTables(ctx, database)
		logDatabaseOperation(ctx, "get_tables", started, map[string]any{
			"database":  database,
			"cache_hit": false,
		}, err)
		return tables, err
	})
	if done == nil {
		logger.DebugOperation("get_tables", time.Now(), map[string]any{
			"database":    database,
			"cache_hit":   true,
			"cache_scope": "connection",
		})
	}
	return key, done
}

func (loader *schemaLoader) loadTables(ctx context.Context, database string) (map[string][]string, error) {
	if loader == nil || loader.driver == nil {
		return nil, errors.New("schema loader driver is nil")
	}

	key, done := loader.requestTables(ctx, database)
	if done != nil {
		<-done
	}

	status, value, err := loader.cache.result(key)
	if err != nil {
		return nil, err
	}
	if status != MetadataReady {
		return nil, errors.New("schema table list is not ready")
	}

	tables, ok := value.(map[string][]string)
	if !ok {
		return nil, errors.New("schema table list has an invalid result")
	}
	return copySchemaTables(tables), nil
}

func (loader *schemaLoader) invalidateAll() {
	if loader == nil || loader.cache == nil {
		return
	}
	loader.cache.invalidateAll()
}

func copySchemaTables(tables map[string][]string) map[string][]string {
	copied := make(map[string][]string, len(tables))
	for schema, names := range tables {
		copied[schema] = append([]string(nil), names...)
	}
	return copied
}

// editorTableNames keeps the familiar bare-name completion for unique tables,
// while exposing schema-qualified names when schemas contain duplicates.
func editorTableNames(tables []editorSchemaTable) []string {
	counts := make(map[string]int, len(tables))
	for _, table := range tables {
		counts[strings.ToLower(table.bareName)]++
	}

	names := make([]string, 0, len(tables))
	seen := make(map[string]struct{}, len(tables))
	for _, table := range tables {
		name := table.bareName
		if counts[strings.ToLower(table.bareName)] > 1 {
			name = table.qualifiedName
		}
		key := strings.ToLower(name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		names = append(names, name)
	}
	return names
}

// visibleTables converts the driver's table map into deterministic table
// descriptors and excludes configured schemas before any column work begins.
func (loader *schemaLoader) visibleTables(_ string, tables map[string][]string, schemas []string) []editorSchemaTable {
	if loader == nil || loader.driver == nil {
		return nil
	}

	allowedSchemas := make(map[string]struct{}, len(schemas))
	for _, schema := range schemas {
		allowedSchemas[schema] = struct{}{}
	}

	var visible []editorSchemaTable
	for schema, names := range tables {
		if loader.driver.UseSchemas() && len(allowedSchemas) > 0 {
			if _, ok := allowedSchemas[schema]; !ok {
				continue
			}
		}

		for _, name := range names {
			qualifiedName := name
			if loader.driver.UseSchemas() && schema != "" {
				qualifiedName = schema + "." + name
			}
			visible = append(visible, editorSchemaTable{
				bareName:      name,
				qualifiedName: qualifiedName,
			})
		}
	}

	sort.Slice(visible, func(i, j int) bool {
		if visible[i].bareName != visible[j].bareName {
			return visible[i].bareName < visible[j].bareName
		}
		return visible[i].qualifiedName < visible[j].qualifiedName
	})
	return visible
}

func (loader *schemaLoader) requestColumns(ctx context.Context, database, table string) (metadataKey, <-chan struct{}) {
	key := newMetadataKey(database, table, MetadataColumns)
	if loader == nil || loader.driver == nil {
		return key, nil
	}

	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	done := loader.cache.requestWithContext(ctx, key, cancel, func() (any, error) {
		started := time.Now()
		columns, err := loader.driver.GetTableColumns(ctx, database, table)
		logDatabaseOperation(ctx, "get_table_columns", started, map[string]any{
			"database":  database,
			"table":     table,
			"cache_hit": false,
		}, err)
		return columns, err
	})
	if done == nil {
		logger.DebugOperation("get_table_columns", time.Now(), map[string]any{
			"database":    database,
			"table":       table,
			"cache_hit":   true,
			"cache_scope": "connection",
		})
	}
	return key, done
}

// preloadEditorColumns eagerly loads only small visible schemas. A driver
// bulk capability is optional; without it the shared loader leaves columns
// lazy rather than recreating a per-table N+1 query pattern.
func (loader *schemaLoader) preloadEditorColumns(ctx context.Context, database string, tables []editorSchemaTable, threshold int, publish func(editorSchemaTable, []string)) {
	if loader == nil || loader.driver == nil || len(tables) == 0 {
		return
	}

	cache := loader.cache
	generation := cache.generationValue()
	pending := make([]editorSchemaTable, 0, len(tables))
	for _, table := range tables {
		key := newMetadataKey(database, table.qualifiedName, MetadataColumns)
		status, value, _ := cache.result(key)
		switch status {
		case MetadataReady:
			if publish != nil {
				publish(table, editorColumnNames(value))
			}
		case MetadataLoading:
			done := cache.completion(key)
			if done != nil {
				go loader.publishWhenReady(key, table, done, publish)
				continue
			}
			// The request may have completed between result and completion.
			status, value, err := cache.result(key)
			if err == nil && status == MetadataReady && publish != nil {
				publish(table, editorColumnNames(value))
			}
		default:
			pending = append(pending, table)
		}
	}

	// A threshold only controls new network work. Already cached or in-flight
	// columns above are still useful to autocomplete, even for lazy schemas.
	if len(pending) == 0 || threshold <= 0 || len(tables) > threshold {
		return
	}

	bulkLoader, ok := loader.driver.(drivers.BulkTableColumnLoader)
	if !ok {
		// Unsupported drivers remain lazy. A specific completion request still
		// uses requestColumns and fetches one table immediately.
		return
	}

	names := make([]string, 0, len(pending))
	for _, table := range pending {
		names = append(names, table.qualifiedName)
	}
	started := time.Now()
	bulkResults, err := bulkLoader.GetTableColumnsBulk(ctx, database, names)
	logDatabaseOperation(ctx, "get_table_columns_bulk", started, map[string]any{
		"database": database,
		"tables":   len(names),
	}, err)
	if err != nil {
		logger.Error("Failed to bulk load table columns for editor autocomplete", map[string]any{
			"database": database,
			"tables":   len(names),
			"error":    err,
		})
		return
	}

	for _, table := range pending {
		key := newMetadataKey(database, table.qualifiedName, MetadataColumns)
		value, found := bulkColumnResult(bulkResults, table)
		if !found {
			// A successful catalog query with no row for a requested table is a
			// ready empty result. This prevents repeatedly asking for a table
			// whose columns are inaccessible or genuinely empty.
			value = [][]string{}
		}
		if ctx != nil && ctx.Err() != nil {
			return
		}
		if !cache.storeResult(key, value, nil, &generation) {
			return
		}
		// A concurrent per-table refresh may already have published a newer
		// ready value. Publish the value actually accepted by the cache.
		status, cached, err := cache.result(key)
		if publish != nil && status == MetadataReady && err == nil {
			publish(table, editorColumnNames(cached))
		}
	}
}

func (loader *schemaLoader) publishWhenReady(key metadataKey, table editorSchemaTable, done <-chan struct{}, publish func(editorSchemaTable, []string)) {
	<-done
	status, value, err := loader.cache.result(key)
	if err == nil && status == MetadataReady && publish != nil {
		publish(table, editorColumnNames(value))
	}
}

func bulkColumnResult(results map[string][][]string, table editorSchemaTable) ([][]string, bool) {
	if value, ok := results[table.qualifiedName]; ok {
		return value, true
	}
	qualifiedName := strings.ToLower(table.qualifiedName)
	for name, value := range results {
		if strings.ToLower(name) == qualifiedName {
			return value, true
		}
	}

	// A bare result is safe only when the requested table is itself bare. A
	// schema-qualified request must never borrow another schema's bare bucket.
	if table.qualifiedName == table.bareName {
		if value, ok := results[table.bareName]; ok {
			return value, true
		}
		bareName := strings.ToLower(table.bareName)
		for name, value := range results {
			if strings.ToLower(name) == bareName {
				return value, true
			}
		}
	}
	return nil, false
}

func editorColumnNames(value any) []string {
	if names, ok := value.([]string); ok {
		return append([]string(nil), names...)
	}

	rows, ok := value.([][]string)
	if !ok || len(rows) < 2 {
		return []string{}
	}

	names := make([]string, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if len(row) > 0 && row[0] != "" {
			names = append(names, row[0])
		}
	}
	return names
}
