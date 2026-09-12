package components

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
)

// MetadataKind identifies one independently loaded piece of table metadata.
type MetadataKind string

const (
	MetadataColumns     MetadataKind = "columns"
	MetadataPrimaryKeys MetadataKind = "primary_keys"
	MetadataForeignKeys MetadataKind = "foreign_keys"
	MetadataConstraints MetadataKind = "constraints"
	MetadataIndexes     MetadataKind = "indexes"
)

var metadataKinds = []MetadataKind{
	MetadataColumns,
	MetadataPrimaryKeys,
	MetadataForeignKeys,
	MetadataConstraints,
	MetadataIndexes,
}

func newMetadataStates() map[MetadataKind]MetadataState {
	states := make(map[MetadataKind]MetadataState, len(metadataKinds))
	for _, kind := range metadataKinds {
		states[kind] = MetadataUnloaded
	}
	return states
}

// MetadataState describes the lifecycle of one metadata request.
type MetadataState uint8

const (
	MetadataUnloaded MetadataState = iota
	MetadataLoading
	MetadataReady
	MetadataFailed
)

func (state MetadataState) String() string {
	switch state {
	case MetadataLoading:
		return "loading"
	case MetadataReady:
		return "ready"
	case MetadataFailed:
		return "failed"
	default:
		return "unloaded"
	}
}

type metadataKey struct {
	database string
	schema   string
	table    string
	kind     MetadataKind
}

func newMetadataKey(database, table string, kind MetadataKind) metadataKey {
	schema, tableName := splitMetadataTableName(table)
	return metadataKey{
		database: database,
		schema:   schema,
		table:    tableName,
		kind:     kind,
	}
}

func splitMetadataTableName(table string) (schema, tableName string) {
	table = strings.TrimSpace(table)
	if dot := strings.IndexByte(table, '.'); dot >= 0 {
		return strings.TrimSpace(table[:dot]), strings.TrimSpace(table[dot+1:])
	}
	return "", table
}

type metadataCacheEntry struct {
	status MetadataState
	value  any
	err    error
	done   chan struct{}
}

// metadataCache is scoped to one connected Home/database connection. Entries
// remain available until that Home is discarded; failed entries can be retried
// while ready entries are reused without another database call.
type metadataCache struct {
	mu      sync.Mutex
	entries map[metadataKey]*metadataCacheEntry
}

func newMetadataCache() *metadataCache {
	return &metadataCache{entries: make(map[metadataKey]*metadataCacheEntry)}
}

// request returns a completion channel for a new or in-flight request. A nil
// channel means the value is already ready in the cache.
func (cache *metadataCache) request(key metadataKey, load func() (any, error)) <-chan struct{} {
	cache.mu.Lock()
	if entry, ok := cache.entries[key]; ok {
		switch entry.status {
		case MetadataReady:
			cache.mu.Unlock()
			return nil
		case MetadataLoading:
			done := entry.done
			cache.mu.Unlock()
			return done
		}
	}

	entry := &metadataCacheEntry{
		status: MetadataLoading,
		done:   make(chan struct{}),
	}
	cache.entries[key] = entry
	cache.mu.Unlock()

	go func() {
		value, err := load()
		cache.mu.Lock()
		defer cache.mu.Unlock()

		// The entry can only be replaced after a request has finished, but keep
		// this guard so a future invalidation cannot close the wrong request.
		if current, ok := cache.entries[key]; !ok || current != entry {
			return
		}

		entry.value = value
		entry.err = err
		if err != nil {
			entry.status = MetadataFailed
		} else {
			entry.status = MetadataReady
		}
		close(entry.done)
	}()

	return entry.done
}

func (cache *metadataCache) result(key metadataKey) (MetadataState, any, error) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	entry, ok := cache.entries[key]
	if !ok {
		return MetadataUnloaded, nil, nil
	}
	return entry.status, entry.value, entry.err
}

func metadataCacheForHome(home *Home) *metadataCache {
	if home == nil {
		return newMetadataCache()
	}
	return home.metadataCacheForConnection()
}

func (home *Home) metadataCacheForConnection() *metadataCache {
	home.metadataCacheMu.Lock()
	defer home.metadataCacheMu.Unlock()
	if home.metadataCache == nil {
		home.metadataCache = newMetadataCache()
	}
	return home.metadataCache
}

func (table *ResultsTable) requestMetadata(databaseName, tableName string, kind MetadataKind) (metadataKey, <-chan struct{}) {
	key := newMetadataKey(databaseName, tableName, kind)
	cache := table.metadataCacheForTable()
	done := cache.request(key, func() (any, error) {
		switch kind {
		case MetadataColumns:
			return table.DBDriver.GetTableColumns(databaseName, tableName)
		case MetadataPrimaryKeys:
			return table.DBDriver.GetPrimaryKeyColumnNames(databaseName, tableName)
		case MetadataForeignKeys:
			return table.DBDriver.GetForeignKeys(databaseName, tableName)
		case MetadataConstraints:
			return table.DBDriver.GetConstraints(databaseName, tableName)
		case MetadataIndexes:
			return table.DBDriver.GetIndexes(databaseName, tableName)
		default:
			return nil, fmt.Errorf("unknown metadata kind %q", kind)
		}
	})
	return key, done
}

func (table *ResultsTable) metadataCacheForTable() *metadataCache {
	table.metadataCacheMu.Lock()
	defer table.metadataCacheMu.Unlock()

	if table.metadataCache == nil {
		if table.Home != nil {
			table.metadataCache = table.Home.metadataCacheForConnection()
		} else {
			table.metadataCache = newMetadataCache()
		}
	}
	return table.metadataCache
}

func (table *ResultsTable) GetMetadataState(kind MetadataKind) MetadataState {
	if table.state == nil {
		return MetadataUnloaded
	}

	table.state.metadataMu.RLock()
	defer table.state.metadataMu.RUnlock()
	if table.state.metadataStates == nil {
		return MetadataUnloaded
	}
	return table.state.metadataStates[kind]
}

func (table *ResultsTable) GetMetadataError(kind MetadataKind) error {
	if table.state == nil {
		return nil
	}

	table.state.metadataMu.RLock()
	defer table.state.metadataMu.RUnlock()
	return table.state.metadataErrors[kind]
}

func (table *ResultsTable) setMetadataState(kind MetadataKind, status MetadataState, err error) {
	if table.state == nil {
		return
	}

	table.state.metadataMu.Lock()
	defer table.state.metadataMu.Unlock()
	if table.state.metadataStates == nil {
		table.state.metadataStates = make(map[MetadataKind]MetadataState)
	}
	if table.state.metadataErrors == nil {
		table.state.metadataErrors = make(map[MetadataKind]error)
	}
	table.state.metadataStates[kind] = status
	if status == MetadataFailed {
		table.state.metadataErrors[kind] = err
	} else {
		delete(table.state.metadataErrors, kind)
	}
}

func (table *ResultsTable) loadMetadataKind(ctx context.Context, generation uint64, databaseName, tableName string, kind MetadataKind) {
	key, done := table.requestMetadata(databaseName, tableName, kind)
	if done != nil {
		table.setMetadataState(kind, MetadataLoading, nil)
	}
	table.queueMetadataResult(ctx, generation, databaseName, tableName, kind, key, done)
}

func (table *ResultsTable) queueMetadataResult(ctx context.Context, generation uint64, databaseName, tableName string, kind MetadataKind, key metadataKey, done <-chan struct{}) {
	go func() {
		if done != nil {
			<-done
		}
		if !table.isCurrentLoad(ctx, generation) {
			return
		}

		cache := table.metadataCacheForTable()
		App.QueueUpdateDraw(func() {
			status, value, err := cache.result(key)
			table.applyMetadataResult(ctx, generation, databaseName, tableName, kind, status, value, err)
		})
	}()
}

func (table *ResultsTable) updatePrimaryKeyIndicator(primaryKeyColumnNames []string) {
	if len(primaryKeyColumnNames) != 0 || table.Pagination == nil {
		return
	}

	currentText := table.Pagination.textView.GetText(false)
	if !strings.Contains(currentText, "⚠ No Primary Key") {
		table.Pagination.textView.SetText(currentText + " ⚠ No Primary Key")
	}
}

// applyMetadataResult is deliberately separate from queueMetadataResult so the
// generation and table identity checks are made immediately before mutating
// the visible table. The cache is still updated by the worker even when this
// returns false for a stale view.
func (table *ResultsTable) applyMetadataResult(ctx context.Context, generation uint64, databaseName, tableName string, kind MetadataKind, status MetadataState, value any, err error) bool {
	if !table.isCurrentLoad(ctx, generation) || table.GetDatabaseName() != databaseName || table.GetTableName() != tableName {
		return false
	}

	table.setMetadataState(kind, status, err)
	if status != MetadataReady {
		return true
	}

	switch kind {
	case MetadataColumns:
		columns, _ := value.([][]string)
		table.SetColumns(columns)
		table.updateMetadataRows(2, columns)
	case MetadataPrimaryKeys:
		primaryKeyColumnNames, _ := value.([]string)
		table.SetPrimaryKeyColumnNames(primaryKeyColumnNames)
		table.updatePrimaryKeyIndicator(primaryKeyColumnNames)
	case MetadataForeignKeys:
		foreignKeys, _ := value.([][]string)
		table.SetForeignKeys(foreignKeys)
		if table.Menu != nil && table.Menu.GetSelectedOption() == 4 {
			table.UpdateRows(foreignKeys)
		} else {
			table.UpdateRowsColor(app.Styles.PrimaryTextColor, tview.Styles.PrimaryTextColor)
		}
		App.ForceDraw()
	case MetadataConstraints:
		constraints, _ := value.([][]string)
		table.SetConstraints(constraints)
		table.updateMetadataRows(3, constraints)
	case MetadataIndexes:
		indexes, _ := value.([][]string)
		table.SetIndexes(indexes)
		table.updateMetadataRows(5, indexes)
	}
	return true
}
