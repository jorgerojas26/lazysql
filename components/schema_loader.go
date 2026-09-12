package components

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/jorgerojas26/lazysql/drivers"
	"github.com/jorgerojas26/lazysql/helpers/logger"
)

// editorSchemaTable is the driver-facing name and the autocomplete-facing
// name for one visible table. PostgreSQL needs the qualified name for catalog
// lookups, while the editor traditionally displays the bare table name.
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

func (loader *schemaLoader) requestTables(ctx context.Context, database string) (metadataKey, <-chan struct{}) {
	key := newMetadataKey(database, "", MetadataTables)
	if loader == nil || loader.driver == nil {
		return key, nil
	}

	done := loader.cache.request(key, func() (any, error) {
		return loader.driver.GetTables(ctx, database)
	})
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

func (loader *schemaLoader) invalidateTables(database string) {
	if loader == nil || loader.cache == nil || database == "" {
		return
	}
	loader.cache.invalidate(newMetadataKey(database, "", MetadataTables))
}

func (loader *schemaLoader) invalidateAll() {
	if loader == nil || loader.cache == nil {
		return
	}
	loader.cache.invalidateAll()
}

func copySchemaTables(tables map[string][]string) map[string][]string {
	copy := make(map[string][]string, len(tables))
	for schema, names := range tables {
		copy[schema] = append([]string(nil), names...)
	}
	return copy
}

// visibleTables converts the driver's table map into deterministic table
// descriptors and excludes configured schemas before any column work begins.
func (loader *schemaLoader) visibleTables(database string, tables map[string][]string, schemas []string) []editorSchemaTable {
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
			if loader.driver.UseSchemas() && schema != "" && schema != database {
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

	done := loader.cache.request(key, func() (any, error) {
		return loader.driver.GetTableColumns(ctx, database, table)
	})
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
	bulkResults, err := bulkLoader.GetTableColumnsBulk(ctx, database, names)
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
		cache.store(key, value, nil)
		if publish != nil {
			publish(table, editorColumnNames(value))
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
	if value, ok := results[table.bareName]; ok {
		return value, true
	}

	qualifiedName := strings.ToLower(table.qualifiedName)
	bareName := strings.ToLower(table.bareName)
	for name, value := range results {
		switch strings.ToLower(name) {
		case qualifiedName, bareName:
			return value, true
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
