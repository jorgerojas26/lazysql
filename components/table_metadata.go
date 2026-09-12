package components

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/rivo/tview"

	"github.com/jorgerojas26/lazysql/app"
	"github.com/jorgerojas26/lazysql/helpers/logger"
)

// MetadataKind identifies one independently loaded piece of table metadata.
type MetadataKind string

const (
	// MetadataTables is the shared schema-list cache entry. It is deliberately
	// not part of metadataKinds because Records only loads per-table metadata.
	MetadataTables      MetadataKind = "tables"
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
	status     MetadataState
	value      any
	err        error
	done       chan struct{}
	doneClosed bool
	ctx        context.Context
	cancel     context.CancelFunc
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
	return cache.requestWithContext(key, nil, nil, load)
}

// requestWithContext associates an optional cancellation context with the
// underlying cache entry. The context belongs to the database work, not to a
// particular Records/surface load generation, so invalidating one entry can
// stop only that query while unrelated metadata continues in flight.
func (cache *metadataCache) requestWithContext(key metadataKey, ctx context.Context, cancel context.CancelFunc, load func() (any, error)) <-chan struct{} {
	var staleCancel context.CancelFunc

	cache.mu.Lock()
	if entry, ok := cache.entries[key]; ok {
		switch entry.status {
		case MetadataReady:
			cache.mu.Unlock()
			if cancel != nil {
				cancel()
			}
			return nil
		case MetadataLoading:
			if entry.ctx == nil || entry.ctx.Err() == nil {
				done := entry.done
				cache.mu.Unlock()
				if cancel != nil {
					cancel()
				}
				return done
			}

			// An identity-scoped context can be canceled just before a table is
			// revisited. Do not let the new consumer inherit that doomed request.
			delete(cache.entries, key)
			if !entry.doneClosed {
				close(entry.done)
				entry.doneClosed = true
			}
			staleCancel = entry.cancel
			entry.cancel = nil
		}
	}

	entry := &metadataCacheEntry{
		status: MetadataLoading,
		done:   make(chan struct{}),
		ctx:    ctx,
		cancel: cancel,
	}
	cache.entries[key] = entry
	cache.mu.Unlock()

	if staleCancel != nil {
		staleCancel()
	}

	go func() {
		value, err := load()
		cache.mu.Lock()
		defer cache.mu.Unlock()

		// A refresh can replace an in-flight entry. Its result must not replace
		// the newer request, but any waiters on the old request still need to be
		// released.
		if current, ok := cache.entries[key]; !ok || current != entry {
			if !entry.doneClosed {
				close(entry.done)
				entry.doneClosed = true
			}
			return
		}

		entry.value = value
		entry.err = err
		if err != nil {
			entry.status = MetadataFailed
		} else {
			entry.status = MetadataReady
		}
		entry.cancel = nil
		if !entry.doneClosed {
			close(entry.done)
			entry.doneClosed = true
		}
	}()

	return entry.done
}

// invalidate removes one metadata entry so the next request performs a fresh
// database lookup. The cache releases an in-flight completion, cancels only the
// invalidated query, and ignores its result if a replacement request wins the
// race.
func (cache *metadataCache) invalidate(key metadataKey) {
	cache.mu.Lock()
	entry, ok := cache.entries[key]
	if !ok {
		cache.mu.Unlock()
		return
	}
	delete(cache.entries, key)
	if !entry.doneClosed {
		close(entry.done)
		entry.doneClosed = true
	}
	cancel := entry.cancel
	entry.cancel = nil
	cache.mu.Unlock()

	if cancel != nil {
		cancel()
	}
}

// invalidateAll drops every schema and table-metadata entry for a connection.
// In-flight waiters are released and their results are ignored by request's
// entry-identity check, allowing a replacement tree/schema load to start
// immediately after a successful DDL statement.
func (cache *metadataCache) invalidateAll() {
	if cache == nil {
		return
	}

	cache.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(cache.entries))
	for key, entry := range cache.entries {
		delete(cache.entries, key)
		if !entry.doneClosed {
			close(entry.done)
			entry.doneClosed = true
		}
		if entry.cancel != nil {
			cancels = append(cancels, entry.cancel)
			entry.cancel = nil
		}
	}
	cache.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
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

// completion returns the done channel for an in-flight request without
// starting a request for an unloaded key. This lets secondary consumers join
// work already started by another consumer while keeping lazy keys lazy.
func (cache *metadataCache) completion(key metadataKey) <-chan struct{} {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	entry, ok := cache.entries[key]
	if !ok || entry.status != MetadataLoading {
		return nil
	}
	return entry.done
}

// store publishes a result obtained by a bulk loader under an individual
// metadata key. Replacing a loading entry is safe: the old request checks its
// entry identity before publishing, and its waiters are released here.
func (cache *metadataCache) store(key metadataKey, value any, err error) {
	cache.mu.Lock()

	var cancel context.CancelFunc
	if current, ok := cache.entries[key]; ok {
		if current.status == MetadataReady {
			cache.mu.Unlock()
			return
		}
		if current.status == MetadataLoading && !current.doneClosed {
			close(current.done)
			current.doneClosed = true
		}
		cancel = current.cancel
		current.cancel = nil
	}

	entry := &metadataCacheEntry{
		status: MetadataReady,
		value:  value,
		err:    err,
		done:   make(chan struct{}),
	}
	if err != nil {
		entry.status = MetadataFailed
	}
	close(entry.done)
	entry.doneClosed = true
	cache.entries[key] = entry
	cache.mu.Unlock()

	if cancel != nil {
		cancel()
	}
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
		if home.schemaLoader != nil && home.schemaLoader.cache != nil {
			home.metadataCache = home.schemaLoader.cache
		} else {
			home.metadataCache = newMetadataCache()
		}
	}
	return home.metadataCache
}

func (table *ResultsTable) requestMetadata(databaseName, tableName string, kind MetadataKind) (metadataKey, <-chan struct{}) {
	return table.requestMetadataWithContext(context.Background(), databaseName, tableName, kind)
}

func (table *ResultsTable) requestMetadataWithContext(ctx context.Context, databaseName, tableName string, kind MetadataKind) (metadataKey, <-chan struct{}) {
	key := newMetadataKey(databaseName, tableName, kind)
	cache := table.metadataCacheForTable()
	if ctx == nil {
		ctx = context.Background()
	}
	metadataCtx, metadataCancel := context.WithCancel(ctx)
	done := cache.requestWithContext(key, metadataCtx, metadataCancel, func() (any, error) {
		started := time.Now()
		var value any
		var err error
		switch kind {
		case MetadataColumns:
			value, err = table.DBDriver.GetTableColumns(metadataCtx, databaseName, tableName)
		case MetadataPrimaryKeys:
			value, err = table.DBDriver.GetPrimaryKeyColumnNames(metadataCtx, databaseName, tableName)
		case MetadataForeignKeys:
			value, err = table.DBDriver.GetForeignKeys(metadataCtx, databaseName, tableName)
		case MetadataConstraints:
			value, err = table.DBDriver.GetConstraints(metadataCtx, databaseName, tableName)
		case MetadataIndexes:
			value, err = table.DBDriver.GetIndexes(metadataCtx, databaseName, tableName)
		default:
			err = fmt.Errorf("unknown metadata kind %q", kind)
		}
		logDatabaseOperation("get_"+string(kind), started, metadataCtx, map[string]any{
			"database":  databaseName,
			"table":     tableName,
			"kind":      kind,
			"cache_hit": false,
		}, err)
		return value, err
	})
	return key, done
}

func (table *ResultsTable) metadataCacheForTable() *metadataCache {
	table.metadataCacheMu.Lock()
	defer table.metadataCacheMu.Unlock()

	if table.metadataCache == nil {
		if table.Home != nil {
			table.metadataCache = table.Home.metadataCacheForConnection()
		} else if table.Tree != nil && table.Tree.schemaLoader != nil {
			table.metadataCache = table.Tree.schemaLoader.cache
		} else {
			table.metadataCache = newMetadataCache()
		}
	}
	return table.metadataCache
}

func (table *ResultsTable) metadataIdentityGenerationValue() uint64 {
	table.metadataIdentityMu.RLock()
	defer table.metadataIdentityMu.RUnlock()
	return table.metadataIdentityGeneration
}

// metadataContextForIdentityLocked returns a context shared by structural
// metadata requests for one table identity. The caller must hold
// metadataApplyMu so identity transitions cannot race context selection.
func (table *ResultsTable) metadataContextForIdentityLocked(identityGeneration uint64) context.Context {
	if table.metadataContext != nil && table.metadataContextGeneration == identityGeneration {
		return table.metadataContext
	}

	table.cancelMetadataContextLocked()
	base := context.Background()
	if app.App != nil {
		base = app.App.Context()
	}
	ctx, cancel := context.WithCancel(base)
	table.metadataContext = ctx
	table.metadataContextCancel = cancel
	table.metadataContextGeneration = identityGeneration
	return ctx
}

func (table *ResultsTable) cancelMetadataContextLocked() {
	if table.metadataContextCancel != nil {
		table.metadataContextCancel()
	}
	table.metadataContext = nil
	table.metadataContextCancel = nil
}

func (table *ResultsTable) cancelMetadataContext() {
	table.metadataApplyMu.Lock()
	table.cancelMetadataContextLocked()
	table.metadataApplyMu.Unlock()
}

func (table *ResultsTable) isCurrentMetadataIdentity(generation uint64, databaseName, tableName string) bool {
	table.metadataIdentityMu.RLock()
	defer table.metadataIdentityMu.RUnlock()
	return table.metadataIdentityGeneration == generation &&
		table.state.databaseName == databaseName &&
		table.state.tableName == tableName
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

func (table *ResultsTable) loadMetadataKind(ctx context.Context, generation uint64, databaseName, tableName string, kind MetadataKind) <-chan struct{} {
	identityGeneration := table.metadataIdentityGenerationValue()
	if !table.isCurrentLoad(ctx, generation) {
		return nil
	}

	// Metadata consumers outlive Records and surface loads. Only a table
	// identity change makes a pending result stale. Keep request creation and
	// identity-context selection under the same lock as identity transitions so
	// an old request cannot start after its table became stale.
	table.metadataApplyMu.Lock()
	if !table.isCurrentLoad(ctx, generation) || !table.isCurrentMetadataIdentity(identityGeneration, databaseName, tableName) {
		table.metadataApplyMu.Unlock()
		return nil
	}
	metadataCtx := table.metadataContextForIdentityLocked(identityGeneration)
	key, done := table.requestMetadataWithContext(metadataCtx, databaseName, tableName, kind)
	if done != nil {
		table.setMetadataState(kind, MetadataLoading, nil)
	}
	table.metadataApplyMu.Unlock()

	if done == nil {
		logger.DebugOperation("metadata_cache_lookup", time.Now(), map[string]any{
			"database":  databaseName,
			"table":     tableName,
			"kind":      kind,
			"cache_hit": true,
		})
	} else {
		logger.DebugOperation("metadata_cache_lookup", time.Now(), map[string]any{
			"database":   databaseName,
			"table":      tableName,
			"kind":       kind,
			"cache_wait": true,
		})
	}
	table.queueMetadataResultForIdentity(identityGeneration, databaseName, tableName, kind, key, done)
	return done
}

// RefreshMetadata invalidates and reloads exactly one structural metadata kind.
// It deliberately leaves the previous value in ResultsTableState until a new
// result arrives, so a failed refresh cannot destroy usable data.
func (table *ResultsTable) RefreshMetadata(kind MetadataKind) {
	if !isRefreshableMetadataKind(kind) || table.DBDriver == nil {
		return
	}

	databaseName := table.GetDatabaseName()
	tableName := table.GetTableName()
	if databaseName == "" || tableName == "" {
		return
	}

	ctx, generation := table.startLoad()
	key := newMetadataKey(databaseName, tableName, kind)
	cache := table.metadataCacheForTable()
	cache.invalidate(key)
	table.setMetadataState(kind, MetadataUnloaded, nil)
	done := table.loadMetadataKind(ctx, generation, databaseName, tableName, kind)

	go func() {
		if done != nil {
			<-done
		}
		App.QueueUpdateDraw(func() {
			if table.isCurrentLoad(ctx, generation) {
				table.SetLoading(false)
			}
		})
	}()
}

func isRefreshableMetadataKind(kind MetadataKind) bool {
	switch kind {
	case MetadataColumns, MetadataForeignKeys, MetadataConstraints, MetadataIndexes:
		return true
	default:
		return false
	}
}

func metadataKindForMenuOption(option int) (MetadataKind, bool) {
	switch option {
	case 2:
		return MetadataColumns, true
	case 3:
		return MetadataConstraints, true
	case 4:
		return MetadataForeignKeys, true
	case 5:
		return MetadataIndexes, true
	default:
		return "", false
	}
}

func metadataDisplayName(kind MetadataKind) string {
	switch kind {
	case MetadataColumns:
		return menuColumns
	case MetadataForeignKeys:
		return menuForeignKeys
	case MetadataConstraints:
		return menuConstraints
	case MetadataIndexes:
		return menuIndexes
	case MetadataPrimaryKeys:
		return "Primary Keys"
	default:
		return "Metadata"
	}
}

// showMetadataSurface keeps failed metadata local to its own tab. The backing
// metadata value remains untouched, while the user gets an explicit retry hint.
func (table *ResultsTable) showMetadataSurface(kind MetadataKind) {
	if table.GetMetadataState(kind) == MetadataFailed {
		table.UpdateRows([][]string{{fmt.Sprintf("%s unavailable — press R to retry", metadataDisplayName(kind))}})
		return
	}

	var rows [][]string
	switch kind {
	case MetadataColumns:
		rows = table.GetColumns()
	case MetadataForeignKeys:
		rows = table.GetForeignKeys()
	case MetadataConstraints:
		rows = table.GetConstraints()
	case MetadataIndexes:
		rows = table.GetIndexes()
	}
	table.UpdateRows(rows)
}

// queueMetadataResult is kept as a compatibility wrapper for callers that
// already have a Records load token. Metadata delivery itself is keyed to the
// table identity, not that token, so a same-table refresh cannot abandon it.
func (table *ResultsTable) queueMetadataResult(_ context.Context, _ uint64, databaseName, tableName string, kind MetadataKind, key metadataKey, done <-chan struct{}) {
	table.queueMetadataResultForIdentity(table.metadataIdentityGenerationValue(), databaseName, tableName, kind, key, done)
}

// queueMetadataResultForIdentity keeps the cache consumer alive across
// Records/surface load generations while rejecting results from an older table
// identity.
func (table *ResultsTable) queueMetadataResultForIdentity(identityGeneration uint64, databaseName, tableName string, kind MetadataKind, key metadataKey, done <-chan struct{}) {
	go func() {
		if done != nil {
			<-done
		}
		if !table.isCurrentMetadataIdentity(identityGeneration, databaseName, tableName) {
			return
		}

		cache := table.metadataCacheForTable()
		table.queueMetadataUpdate(func() {
			if !table.isCurrentMetadataIdentity(identityGeneration, databaseName, tableName) {
				return
			}
			status, value, err := cache.result(key)
			table.applyMetadataResultForIdentity(identityGeneration, databaseName, tableName, kind, status, value, err)
		})
	}()
}

func (table *ResultsTable) queueMetadataUpdate(update func()) {
	if app.App == nil || app.App.Application == nil {
		update()
		return
	}
	app.App.QueueUpdateDraw(update)
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

// applyMetadataResult rejects a result from an obsolete Records/surface load.
// The metadata queue uses applyMetadataResultForIdentity instead: same-table
// refreshes must preserve its pending consumer.
func (table *ResultsTable) applyMetadataResult(ctx context.Context, generation uint64, databaseName, tableName string, kind MetadataKind, status MetadataState, value any, err error) bool {
	table.metadataApplyMu.Lock()
	defer table.metadataApplyMu.Unlock()
	if !table.isCurrentLoad(ctx, generation) || table.GetDatabaseName() != databaseName || table.GetTableName() != tableName {
		return false
	}
	return table.applyMetadataResultValue(databaseName, tableName, kind, status, value, err)
}

func (table *ResultsTable) applyMetadataResultForIdentity(identityGeneration uint64, databaseName, tableName string, kind MetadataKind, status MetadataState, value any, err error) bool {
	table.metadataApplyMu.Lock()
	defer table.metadataApplyMu.Unlock()
	if !table.isCurrentMetadataIdentity(identityGeneration, databaseName, tableName) {
		return false
	}
	return table.applyMetadataResultValue(databaseName, tableName, kind, status, value, err)
}

func (table *ResultsTable) applyMetadataResultValue(databaseName, tableName string, kind MetadataKind, status MetadataState, value any, err error) bool {
	table.setMetadataState(kind, status, err)
	if status == MetadataFailed {
		logger.Error("Failed to load table metadata", map[string]any{
			"database": databaseName,
			"table":    tableName,
			"kind":     kind,
			"error":    err,
		})
		if table.Menu != nil {
			if selectedKind, ok := metadataKindForMenuOption(table.Menu.GetSelectedOption()); ok && selectedKind == kind {
				table.showMetadataSurface(kind)
			}
		}
		return true
	}
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
