// Package benchmarks contains a deterministic performance-contract harness.
// It runs the real driver, schema-loader, streaming, and CSV-export paths
// against an instrumented credential-free database connector.
package benchmarks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jorgerojas26/lazysql/components"
	"github.com/jorgerojas26/lazysql/drivers"
)

// Scenario identifies one workload in the network-performance matrix.
type Scenario string

const (
	ScenarioSmallTable       Scenario = "small_table"
	ScenarioLargeSlowCount   Scenario = "large_table_slow_count"
	ScenarioMySQLCatalog     Scenario = "mysql_340_catalog"
	ScenarioPagination       Scenario = "pagination"
	ScenarioFilteredRecords  Scenario = "filtered_records"
	ScenarioSorting          Scenario = "sorting"
	ScenarioAutocomplete100  Scenario = "autocomplete_100_tables"
	ScenarioAutocomplete500  Scenario = "autocomplete_500_tables"
	ScenarioAutocomplete2000 Scenario = "autocomplete_2000_tables"
	ScenarioSQLRowCap        Scenario = "sql_results_beyond_row_cap"
	ScenarioSlowRows         Scenario = "slowly_produced_rows"
	ScenarioFullExport       Scenario = "full_csv_export"
)

var defaultScenarios = []Scenario{
	ScenarioSmallTable,
	ScenarioLargeSlowCount,
	ScenarioMySQLCatalog,
	ScenarioPagination,
	ScenarioFilteredRecords,
	ScenarioSorting,
	ScenarioAutocomplete100,
	ScenarioAutocomplete500,
	ScenarioAutocomplete2000,
	ScenarioSQLRowCap,
	ScenarioSlowRows,
	ScenarioFullExport,
}

var defaultRTTs = []time.Duration{0, 50 * time.Millisecond, 100 * time.Millisecond}

// Options controls one harness run. MaxQueryRows follows the application
// contract: a positive value caps interactive SQL results and zero is
// unlimited. A negative value is rejected.
type Options struct {
	RTT                     time.Duration
	PageSize                int
	MaxQueryRows            int
	SchemaBulkLoadThreshold int
	SlowRowDelay            time.Duration
	SlowCountDelay          time.Duration
}

// Result is the machine-readable output for one scenario and one RTT.
type Result struct {
	Scenario               Scenario `json:"scenario"`
	RTTMS                  int64    `json:"rtt_ms"`
	TTFURMS                int64    `json:"ttfur_ms"`
	BlockingRoundTrips     int      `json:"blocking_round_trips"`
	TotalDBOperations      int      `json:"total_db_operations"`
	BackgroundCompletionMS int64    `json:"background_completion_ms"`
	RowsConsumed           int      `json:"rows_consumed"`
	RowsRendered           int      `json:"rows_rendered"`
	BytesConsumed          int64    `json:"bytes_consumed"`
}

// Scenarios returns a copy of the complete benchmark scenario list.
func Scenarios() []Scenario {
	return append([]Scenario(nil), defaultScenarios...)
}

// RTTs returns the default 0/50/100 ms artificial RTT matrix.
func RTTs() []time.Duration {
	return append([]time.Duration(nil), defaultRTTs...)
}

// Run executes every scenario at one artificial RTT using the default
// interactive row cap of 1,000.
func Run(ctx context.Context, rtt time.Duration) ([]Result, error) {
	return RunWithOptions(ctx, defaultOptions(rtt))
}

// RunMatrix executes every scenario for each requested RTT. An empty RTT list
// uses RTTs(). Durations are recorded, but no duration is used as a pass/fail
// criterion by this package.
func RunMatrix(ctx context.Context, rtts []time.Duration) ([]Result, error) {
	if len(rtts) == 0 {
		rtts = RTTs()
	}

	var results []Result
	for _, rtt := range rtts {
		batch, err := Run(ctx, rtt)
		if err != nil {
			return nil, err
		}
		results = append(results, batch...)
	}
	return results, nil
}

// RunScenario executes one named scenario with the default options.
func RunScenario(ctx context.Context, rtt time.Duration, scenario Scenario) (Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if !containsScenario(scenario) {
		return Result{}, fmt.Errorf("unknown benchmark scenario %q", scenario)
	}

	options := defaultOptions(rtt)
	if err := validateOptions(options); err != nil {
		return Result{}, err
	}
	run, err := newScenarioRun(ctx, options, scenario)
	if err != nil {
		return Result{}, fmt.Errorf("%s: %w", scenario, err)
	}
	defer run.close()
	if err := run.execute(); err != nil {
		return Result{}, fmt.Errorf("%s: %w", scenario, err)
	}
	return run.result(), nil
}

// RunWithOptions executes all scenarios without requiring a database service.
// Every scenario uses an instrumented database/sql connector while invoking
// the production MySQL driver and component seams.
func RunWithOptions(ctx context.Context, options Options) ([]Result, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if options.PageSize <= 0 {
		options.PageSize = 300
	}
	if err := validateOptions(options); err != nil {
		return nil, err
	}

	results := make([]Result, 0, len(defaultScenarios))
	for _, scenario := range defaultScenarios {
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		run, err := newScenarioRun(ctx, options, scenario)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", scenario, err)
		}
		executeErr := run.execute()
		result := run.result()
		closeErr := run.close()
		if executeErr != nil {
			return nil, fmt.Errorf("%s: %w", scenario, executeErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("%s: %w", scenario, closeErr)
		}
		results = append(results, result)
	}
	return results, nil
}

func defaultOptions(rtt time.Duration) Options {
	return Options{
		RTT:                     rtt,
		PageSize:                300,
		MaxQueryRows:            1000,
		SchemaBulkLoadThreshold: 200,
		SlowRowDelay:            75 * time.Millisecond,
		SlowCountDelay:          250 * time.Millisecond,
	}
}

func validateOptions(options Options) error {
	if options.RTT < 0 {
		return errors.New("benchmark RTT cannot be negative")
	}
	if options.PageSize < 0 {
		return errors.New("benchmark page size cannot be negative")
	}
	if options.MaxQueryRows < 0 {
		return errors.New("benchmark max query rows cannot be negative")
	}
	if options.SchemaBulkLoadThreshold < 0 {
		return errors.New("benchmark schema bulk-load threshold cannot be negative")
	}
	if options.SlowRowDelay < 0 {
		return errors.New("benchmark slow-row delay cannot be negative")
	}
	if options.SlowCountDelay < 0 {
		return errors.New("benchmark slow-count delay cannot be negative")
	}
	return nil
}

func containsScenario(want Scenario) bool {
	for _, scenario := range defaultScenarios {
		if scenario == want {
			return true
		}
	}
	return false
}

type scenarioRun struct {
	ctx      context.Context
	options  Options
	scenario Scenario
	driver   *drivers.MySQL
	metrics  *observedMetrics

	started    time.Time
	usefulAt   time.Time
	background time.Duration

	mu           sync.Mutex
	rowsRendered int
}

func newScenarioRun(ctx context.Context, options Options, scenario Scenario) (*scenarioRun, error) {
	metrics := &observedMetrics{}
	driver, err := newFixtureDriver(ctx, options, scenario, metrics)
	if err != nil {
		return nil, err
	}
	return &scenarioRun{
		ctx:      ctx,
		options:  options,
		scenario: scenario,
		driver:   driver,
		metrics:  metrics,
		started:  time.Now(),
	}, nil
}

func (run *scenarioRun) close() error {
	if run == nil || run.driver == nil || run.driver.Connection == nil {
		return nil
	}
	return run.driver.Connection.Close()
}

func (run *scenarioRun) execute() error {
	switch run.scenario {
	case ScenarioSmallTable:
		return run.recordsWithBackground("", "", func() error {
			return run.parallelBackground(
				func() error {
					_, err := run.driver.GetTableColumns(run.ctx, benchmarkDatabase, benchmarkRecords)
					return err
				},
				func() error {
					_, err := run.driver.GetPrimaryKeyColumnNames(run.ctx, benchmarkDatabase, benchmarkRecords)
					return err
				},
				func() error {
					_, err := run.driver.GetForeignKeys(run.ctx, benchmarkDatabase, benchmarkRecords)
					return err
				},
				func() error {
					_, err := run.driver.GetConstraints(run.ctx, benchmarkDatabase, benchmarkRecords)
					return err
				},
				func() error {
					_, err := run.driver.GetIndexes(run.ctx, benchmarkDatabase, benchmarkRecords)
					return err
				},
			)
		})
	case ScenarioLargeSlowCount:
		return run.recordsWithBackground("", "", func() error {
			if _, err := run.driver.GetEstimatedRowCount(run.ctx, benchmarkDatabase, benchmarkRecords); err != nil {
				return err
			}
			_, err := run.driver.GetExactRowCount(run.ctx, benchmarkDatabase, benchmarkRecords, "")
			return err
		})
	case ScenarioMySQLCatalog:
		return run.recordsWithBackground("", "", func() error {
			_, err := run.driver.GetForeignKeys(run.ctx, benchmarkDatabase, benchmarkRecords)
			return err
		})
	case ScenarioPagination:
		if err := run.fetchPage(0, "", ""); err != nil {
			return err
		}
		run.markUseful()
		return run.fetchPage(run.options.PageSize, "", "")
	case ScenarioFilteredRecords:
		return run.recordsWithBackground("WHERE category = 'even'", "", nil)
	case ScenarioSorting:
		return run.recordsWithBackground("", "name DESC", nil)
	case ScenarioAutocomplete100:
		return run.autocomplete(100)
	case ScenarioAutocomplete500:
		return run.autocomplete(500)
	case ScenarioAutocomplete2000:
		return run.autocomplete(2000)
	case ScenarioSQLRowCap:
		return run.sqlResults(1500, run.options.MaxQueryRows)
	case ScenarioSlowRows:
		return run.sqlResults(1, 0)
	case ScenarioFullExport:
		return run.fullExport()
	default:
		return fmt.Errorf("unknown benchmark scenario %q", run.scenario)
	}
}

func (run *scenarioRun) recordsWithBackground(where, sort string, background func() error) error {
	if err := run.fetchPage(0, where, sort); err != nil {
		return err
	}
	run.markUseful()
	if background == nil {
		return nil
	}
	started := time.Now()
	if err := background(); err != nil {
		return err
	}
	run.mu.Lock()
	run.background = time.Since(started)
	run.mu.Unlock()
	return nil
}

func (run *scenarioRun) fetchPage(offset int, where, sort string) error {
	_, visibleRows, err := components.FetchRecordsPage(
		run.ctx,
		run.driver,
		benchmarkDatabase,
		benchmarkRecords,
		where,
		sort,
		offset,
		run.options.PageSize,
	)
	if err != nil {
		return err
	}
	run.render(max(len(visibleRows)-1, 0))
	return nil
}

func (run *scenarioRun) autocomplete(tableCount int) error {
	var backgroundStarted time.Time
	result, err := components.LoadEditorSchema(
		run.ctx,
		run.driver,
		benchmarkDatabase,
		run.options.SchemaBulkLoadThreshold,
		func(tableCount int) {
			run.render(tableCount)
			run.markUseful()
			backgroundStarted = time.Now()
		},
	)
	if err != nil {
		return err
	}
	if len(result.TableNames) != tableCount {
		return fmt.Errorf("autocomplete loaded %d tables, want %d", len(result.TableNames), tableCount)
	}
	if !backgroundStarted.IsZero() {
		run.mu.Lock()
		run.background = time.Since(backgroundStarted)
		run.mu.Unlock()
	}
	return nil
}

func (run *scenarioRun) sqlResults(sourceRows, maxRows int) error {
	query := fmt.Sprintf(
		"SELECT id, owner_id, category, name FROM `%s`.`%s` ORDER BY id LIMIT %d",
		benchmarkDatabase,
		benchmarkRecords,
		sourceRows,
	)
	_, err := run.driver.StreamQuery(run.ctx, query, maxRows, func(batch drivers.QueryBatch) error {
		run.render(len(batch.Rows))
		if len(batch.Rows) > 0 {
			run.markUseful()
		}
		return nil
	})
	if err != nil {
		return err
	}
	run.markUseful()
	return nil
}

func (run *scenarioRun) fullExport() error {
	directory, err := os.MkdirTemp("", "lazysql-benchmark-export-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)

	path := filepath.Join(directory, "results.csv")
	query := fmt.Sprintf(
		"SELECT id, owner_id, category, name FROM `%s`.`%s` ORDER BY id",
		benchmarkDatabase,
		benchmarkRecords,
	)
	lastRows := 0
	rows, err := components.ExportAllQueryResults(run.ctx, run.driver, path, query, func(rows int) {
		if delta := rows - lastRows; delta > 0 {
			run.render(delta)
		}
		lastRows = rows
		if rows > 0 {
			run.markUseful()
		}
	})
	if err != nil {
		return err
	}
	if rows > lastRows {
		run.render(rows - lastRows)
	}
	run.markUseful()
	return nil
}

func (run *scenarioRun) parallelBackground(tasks ...func() error) error {
	var wait sync.WaitGroup
	errs := make(chan error, len(tasks))
	for _, task := range tasks {
		wait.Add(1)
		go func(task func() error) {
			defer wait.Done()
			if err := task(); err != nil {
				errs <- err
			}
		}(task)
	}
	wait.Wait()
	close(errs)
	for err := range errs {
		return err
	}
	return nil
}

func (run *scenarioRun) render(rows int) {
	run.mu.Lock()
	run.rowsRendered += rows
	run.mu.Unlock()
}

func (run *scenarioRun) markUseful() {
	run.mu.Lock()
	if run.usefulAt.IsZero() {
		run.usefulAt = time.Now()
	}
	run.mu.Unlock()
}

func (run *scenarioRun) result() Result {
	run.mu.Lock()
	usefulAt := run.usefulAt
	background := run.background
	rendered := run.rowsRendered
	run.mu.Unlock()
	if usefulAt.IsZero() {
		usefulAt = time.Now()
	}
	blocking, total, rows, bytes := run.metrics.snapshot()
	return Result{
		Scenario:               run.scenario,
		RTTMS:                  run.options.RTT.Milliseconds(),
		TTFURMS:                usefulAt.Sub(run.started).Milliseconds(),
		BlockingRoundTrips:     blocking,
		TotalDBOperations:      total,
		BackgroundCompletionMS: background.Milliseconds(),
		RowsConsumed:           rows,
		RowsRendered:           rendered,
		BytesConsumed:          bytes,
	}
}

// WriteJSON writes one machine-readable record per scenario/RTT matrix run.
func WriteJSON(w io.Writer, results []Result) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(results)
}

// WriteTable writes a compact human-readable benchmark report.
func WriteTable(w io.Writer, results []Result) error {
	ordered := append([]Result(nil), results...)
	sort.SliceStable(ordered, func(i, j int) bool {
		if ordered[i].RTTMS != ordered[j].RTTMS {
			return ordered[i].RTTMS < ordered[j].RTTMS
		}
		return ordered[i].Scenario < ordered[j].Scenario
	})
	if _, err := fmt.Fprintln(w, "scenario\trtt_ms\ttfur_ms\tblocking_round_trips\ttotal_db_operations\tbackground_completion_ms\trows_consumed\trows_rendered\tbytes_consumed"); err != nil {
		return err
	}
	for _, result := range ordered {
		if _, err := fmt.Fprintf(w, "%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\n",
			result.Scenario,
			result.RTTMS,
			result.TTFURMS,
			result.BlockingRoundTrips,
			result.TotalDBOperations,
			result.BackgroundCompletionMS,
			result.RowsConsumed,
			result.RowsRendered,
			result.BytesConsumed,
		); err != nil {
			return err
		}
	}
	return nil
}

// ParseRTTs parses comma-separated duration values such as 0ms,50ms,100ms.
func ParseRTTs(value string) ([]time.Duration, error) {
	if strings.TrimSpace(value) == "" {
		return nil, errors.New("at least one RTT is required")
	}
	parts := strings.Split(value, ",")
	rtts := make([]time.Duration, 0, len(parts))
	for _, part := range parts {
		rtt, err := time.ParseDuration(strings.TrimSpace(part))
		if err != nil || rtt < 0 {
			if err == nil {
				err = errors.New("duration cannot be negative")
			}
			return nil, fmt.Errorf("invalid RTT %q: %w", part, err)
		}
		rtts = append(rtts, rtt)
	}
	return rtts, nil
}
