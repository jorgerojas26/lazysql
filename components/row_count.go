package components

import (
	"context"
	"time"

	"github.com/jorgerojas26/lazysql/app"
)

type rowCountKey struct {
	database string
	table    string
	where    string
}

// RefreshRecords invalidates the reusable row-count state before fetching the
// current page again. Structural metadata remains cached by ResultsTable. The
// primary-key metadata request is intentionally limited to missing/failed
// cache entries; other structural metadata is not part of Records refresh.
func (table *ResultsTable) RefreshRecords() {
	table.invalidateRowCount()
	table.fetchRecords(table.GetCurrentSort(), nil, nil, MetadataPrimaryKeys)
}

// RefreshActiveSurface applies R to the surface currently selected in the
// results menu. Records refreshes the page/count state; each metadata tab
// invalidates and reloads only its own cache entry.
func (table *ResultsTable) RefreshActiveSurface() {
	if table.Menu == nil || table.Menu.GetSelectedOption() == 1 {
		table.RefreshRecords()
		return
	}

	if kind, ok := metadataKindForMenuOption(table.Menu.GetSelectedOption()); ok {
		table.RefreshMetadata(kind)
	}
}

func (table *ResultsTable) currentRowCountKey() rowCountKey {
	where := ""
	if table.Filter != nil {
		where = table.Filter.GetCurrentFilter()
	}

	return rowCountKey{
		database: table.GetDatabaseName(),
		table:    table.GetTableName(),
		where:    where,
	}
}

// prepareRowCountIdentity preserves counts while paginating or sorting, but
// starts a new count lifecycle when the Records query identity changes.
func (table *ResultsTable) prepareRowCountIdentity(key rowCountKey) {
	var previousCancel context.CancelFunc

	table.countMu.Lock()
	if table.countKeySet && table.countKey == key {
		table.countMu.Unlock()
		return
	}

	previousCancel = table.countCancel
	table.countCancel = nil
	table.countGeneration++
	table.countKey = key
	table.countKeySet = true
	table.countAttempted = false
	table.countManual = false
	table.countMu.Unlock()

	if previousCancel != nil {
		previousCancel()
	}
	if table.Pagination != nil {
		table.Pagination.ClearCount()
	}
}

// invalidateRowCount is used by Refresh and known data changes. It keeps the
// query identity but clears all reusable count information.
func (table *ResultsTable) invalidateRowCount() {
	var previousCancel context.CancelFunc

	table.countMu.Lock()
	previousCancel = table.countCancel
	table.countCancel = nil
	table.countGeneration++
	table.countAttempted = false
	table.countManual = false
	table.countMu.Unlock()

	if previousCancel != nil {
		previousCancel()
	}
	if table.Pagination != nil {
		table.Pagination.ClearCount()
	}
}

func (table *ResultsTable) countIsCurrent(ctx context.Context, key rowCountKey, generation uint64) bool {
	if ctx == nil || ctx.Err() != nil {
		return false
	}

	table.countMu.Lock()
	defer table.countMu.Unlock()
	return table.countKeySet && table.countKey == key && table.countGeneration == generation && table.countCancel != nil
}

func (table *ResultsTable) finishCount(key rowCountKey, generation uint64) {
	table.countMu.Lock()
	if table.countKeySet && table.countKey == key && table.countGeneration == generation {
		table.countCancel = nil
		table.countManual = false
	}
	table.countMu.Unlock()
}

func (table *ResultsTable) automaticCountContext(key rowCountKey, timeout time.Duration) (context.Context, uint64, context.CancelFunc, bool) {
	if table.Pagination == nil || table.Pagination.HasExactTotal() {
		return nil, 0, nil, false
	}

	var ctx context.Context
	var cancel context.CancelFunc
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(app.App.Context(), timeout)
	} else {
		ctx, cancel = context.WithCancel(app.App.Context())
	}
	var previousCancel context.CancelFunc

	table.countMu.Lock()
	if !table.countKeySet || table.countKey != key || table.countAttempted {
		table.countMu.Unlock()
		cancel()
		return nil, 0, nil, false
	}
	previousCancel = table.countCancel
	table.countGeneration++
	generation := table.countGeneration
	table.countCancel = cancel
	table.countAttempted = true
	table.countManual = false
	table.countMu.Unlock()

	if previousCancel != nil {
		previousCancel()
	}
	return ctx, generation, cancel, true
}

func (table *ResultsTable) startAutomaticRowCount(key rowCountKey) {
	if table.Menu == nil || table.DBDriver == nil || table.Pagination == nil || table.Pagination.HasExactTotal() {
		return
	}

	config := app.App.Config()
	if config == nil || (config.ExactCountTimeoutMS <= 0 && key.where != "") {
		return
	}

	timeout := time.Duration(config.ExactCountTimeoutMS) * time.Millisecond
	ctx, generation, cancel, ok := table.automaticCountContext(key, timeout)
	if !ok {
		return
	}

	go func() {
		defer cancel()

		var estimate *int64
		var estimateErr error
		if key.where == "" {
			estimateStarted := time.Now()
			estimate, estimateErr = table.DBDriver.GetEstimatedRowCount(ctx, key.database, key.table)
			logDatabaseOperation("get_estimated_row_count", estimateStarted, ctx, map[string]any{
				"database":  key.database,
				"table":     key.table,
				"automatic": true,
			}, estimateErr)
			if ctx.Err() != nil {
				table.finishCount(key, generation)
				return
			}

			if estimateErr == nil && estimate != nil && *estimate >= 0 {
				table.queueCountUpdate(func() {
					if table.countIsCurrent(ctx, key, generation) && !table.Pagination.HasExactTotal() {
						table.Pagination.SetEstimatedTotal(*estimate)
					}
				})
			}
		}

		shouldCountExactly := timeout > 0 && (key.where != "" || estimate == nil || estimateErr != nil)
		if timeout > 0 && estimate != nil && estimateErr == nil && config.ExactCountThreshold > 0 {
			shouldCountExactly = *estimate <= int64(config.ExactCountThreshold)
		}
		if !shouldCountExactly || ctx.Err() != nil {
			table.finishCount(key, generation)
			return
		}

		exactStarted := time.Now()
		count, err := table.DBDriver.GetExactRowCount(ctx, key.database, key.table, key.where)
		exactFields := map[string]any{
			"database":  key.database,
			"table":     key.table,
			"automatic": true,
		}
		if ctx.Err() == context.DeadlineExceeded {
			exactFields["cancellation_reason"] = "automatic_timeout"
		}
		logDatabaseOperation("get_exact_row_count", exactStarted, ctx, exactFields, err)
		if err != nil || ctx.Err() != nil {
			// Automatic failures are deliberately silent. The page remains usable
			// and keeps either its estimate or the unknown-more display.
			table.finishCount(key, generation)
			return
		}

		table.queueCountUpdate(func() {
			if table.countIsCurrent(ctx, key, generation) {
				table.Pagination.SetExactTotal(count)
				table.finishCount(key, generation)
			}
		})
	}()
}

func (table *ResultsTable) queueCountUpdate(update func()) {
	if app.App == nil || app.App.Application == nil {
		update()
		return
	}
	app.App.QueueUpdateDraw(update)
}

// ToggleExactCount starts a manual count, or cancels the active manual count.
func (table *ResultsTable) ToggleExactCount() {
	if table.Menu == nil {
		return
	}

	key := table.currentRowCountKey()
	table.prepareRowCountIdentity(key)

	table.countMu.Lock()
	manualActive := table.countManual && table.countCancel != nil
	table.countMu.Unlock()
	if manualActive {
		table.CancelExactCount()
		return
	}

	if table.Pagination == nil || table.Pagination.HasExactTotal() || table.DBDriver == nil {
		return
	}

	ctx, cancel := context.WithCancel(app.App.Context())
	var previousCancel context.CancelFunc
	table.countMu.Lock()
	previousCancel = table.countCancel
	table.countGeneration++
	generation := table.countGeneration
	table.countCancel = cancel
	table.countAttempted = true
	table.countManual = true
	table.countMu.Unlock()

	if previousCancel != nil {
		previousCancel()
	}
	table.Pagination.SetCounting(true)

	go func() {
		started := time.Now()
		count, err := table.DBDriver.GetExactRowCount(ctx, key.database, key.table, key.where)
		logDatabaseOperation("get_exact_row_count", started, ctx, map[string]any{
			"database": key.database,
			"table":    key.table,
			"manual":   true,
		}, err)
		if ctx.Err() != nil {
			table.finishCount(key, generation)
			return
		}

		table.queueCountUpdate(func() {
			if !table.countIsCurrent(ctx, key, generation) {
				return
			}
			if err != nil {
				table.Pagination.SetCountError(err)
			} else {
				table.Pagination.SetExactTotal(count)
			}
			table.finishCount(key, generation)
		})
	}()
}

// StartExactCount is an explicit name for integrations and tests.
func (table *ResultsTable) StartExactCount() {
	table.ToggleExactCount()
}

func (table *ResultsTable) CancelExactCount() {
	var cancel context.CancelFunc

	table.countMu.Lock()
	cancel = table.countCancel
	table.countCancel = nil
	table.countGeneration++
	table.countManual = false
	table.countMu.Unlock()

	if cancel != nil {
		cancel()
	}
	if table.Pagination != nil {
		table.Pagination.ClearCountActivity()
	}
}

func (table *ResultsTable) IsExactCountActive() bool {
	table.countMu.Lock()
	defer table.countMu.Unlock()
	return table.countManual && table.countCancel != nil
}
