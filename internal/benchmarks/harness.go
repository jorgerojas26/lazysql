// Package benchmarks contains a deterministic performance-contract harness.
// It models the database boundaries exercised by the progressive loading
// implementation and adds a configurable delay before each simulated round
// trip. It intentionally does not connect to a real database, so the matrix
// can run in CI without credentials or a service dependency.
package benchmarks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"time"
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
	run := newScenarioRun(ctx, options, scenario)
	if err := run.execute(); err != nil {
		return Result{}, fmt.Errorf("%s: %w", scenario, err)
	}
	return run.result(), nil
}

// RunWithOptions executes all scenarios without requiring a database service.
// It is the seam used by deterministic CI tests.
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
		run := newScenarioRun(ctx, options, scenario)
		if err := run.execute(); err != nil {
			return nil, fmt.Errorf("%s: %w", scenario, err)
		}
		results = append(results, run.result())
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
	ctx        context.Context
	options    Options
	scenario   Scenario
	started    time.Time
	usefulAt   time.Time
	background time.Duration

	mu                 sync.Mutex
	blockingRoundTrips int
	totalOperations    int
	rowsConsumed       int
	rowsRendered       int
	bytesConsumed      int64
}

func newScenarioRun(ctx context.Context, options Options, scenario Scenario) *scenarioRun {
	return &scenarioRun{
		ctx:      ctx,
		options:  options,
		scenario: scenario,
		started:  time.Now(),
	}
}

func (run *scenarioRun) execute() error {
	switch run.scenario {
	case ScenarioSmallTable:
		return run.recordsWithBackground(func() error {
			return run.parallelBackground(
				func() error { return run.operation("get_table_columns", false, 0, 0, 0) },
				func() error { return run.operation("get_primary_keys", false, 0, 0, 0) },
				func() error { return run.operation("get_foreign_keys", false, 0, 0, 0) },
				func() error { return run.operation("get_constraints", false, 0, 0, 0) },
				func() error { return run.operation("get_indexes", false, 0, 0, 0) },
			)
		})
	case ScenarioLargeSlowCount:
		return run.recordsWithBackground(func() error {
			if err := run.operation("get_estimated_row_count", false, 0, 0, 0); err != nil {
				return err
			}
			return run.operation("get_exact_row_count", false, 0, 0, run.options.SlowCountDelay)
		})
	case ScenarioMySQLCatalog:
		return run.recordsWithBackground(func() error {
			// The optimized issue-#340 path is one catalog operation. A
			// fallback is represented by the separate operation name below
			// when a caller builds a fallback-specific run in the future.
			return run.operation("get_foreign_keys_mysql_fast_path", false, 7249, 7249*64, 0)
		})
	case ScenarioPagination:
		if err := run.fetchPage(run.options.PageSize, true); err != nil {
			return err
		}
		run.render(run.options.PageSize)
		run.markUseful()
		if err := run.fetchPage(run.options.PageSize, true); err != nil {
			return err
		}
		run.render(run.options.PageSize)
		return nil
	case ScenarioFilteredRecords, ScenarioSorting:
		return run.recordsWithBackground(nil)
	case ScenarioAutocomplete100:
		return run.autocomplete(100)
	case ScenarioAutocomplete500:
		return run.autocomplete(500)
	case ScenarioAutocomplete2000:
		return run.autocomplete(2000)
	case ScenarioSQLRowCap:
		return run.sqlResults(1500, run.options.MaxQueryRows)
	case ScenarioSlowRows:
		if err := run.operation("stream_query", true, 1, 64, run.options.SlowRowDelay); err != nil {
			return err
		}
		run.render(1)
		run.markUseful()
		return nil
	case ScenarioFullExport:
		const exportRows = 10_000
		if err := run.operation("export_all_query_results", true, exportRows, exportRows*64, 0); err != nil {
			return err
		}
		run.render(exportRows)
		run.markUseful()
		return nil
	default:
		return fmt.Errorf("unknown benchmark scenario %q", run.scenario)
	}
}

func (run *scenarioRun) recordsWithBackground(background func() error) error {
	if err := run.fetchPage(run.options.PageSize, true); err != nil {
		return err
	}
	run.render(run.options.PageSize)
	run.markUseful()
	if background == nil {
		return nil
	}
	started := time.Now()
	if err := background(); err != nil {
		return err
	}
	run.background = time.Since(started)
	return nil
}

func (run *scenarioRun) fetchPage(rows int, blocking bool) error {
	// Page fetches consume a page plus one lookahead row; only the page rows
	// are rendered.
	return run.operation("fetch_records", blocking, rows+1, int64(rows+1)*64, 0)
}

func (run *scenarioRun) autocomplete(tableCount int) error {
	if err := run.operation("get_tables", true, tableCount, int64(tableCount)*48, 0); err != nil {
		return err
	}
	run.render(tableCount)
	run.markUseful()

	if run.options.SchemaBulkLoadThreshold > 0 && tableCount <= run.options.SchemaBulkLoadThreshold {
		started := time.Now()
		if err := run.operation("get_table_columns_bulk", false, tableCount, int64(tableCount)*96, 0); err != nil {
			return err
		}
		run.background = time.Since(started)
		return nil
	}

	started := time.Now()
	if err := run.operation("get_table_columns", false, 1, 96, 0); err != nil {
		return err
	}
	run.background = time.Since(started)
	return nil
}

func (run *scenarioRun) sqlResults(sourceRows, maxRows int) error {
	consumed, rendered := sourceRows, sourceRows
	if maxRows > 0 {
		consumed = maxRows + 1
		if consumed > sourceRows {
			consumed = sourceRows
		}
		rendered = maxRows
		if rendered > sourceRows {
			rendered = sourceRows
		}
	}
	if err := run.operation("stream_query", true, consumed, int64(consumed)*64, 0); err != nil {
		return err
	}
	run.render(rendered)
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

func (run *scenarioRun) operation(_ string, blocking bool, rows int, bytes int64, extra time.Duration) error {
	if err := run.ctx.Err(); err != nil {
		return err
	}

	run.mu.Lock()
	run.totalOperations++
	if blocking {
		run.blockingRoundTrips++
	}
	run.rowsConsumed += rows
	run.bytesConsumed += bytes
	run.mu.Unlock()

	delay := run.options.RTT + extra
	if delay > 0 {
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-run.ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return run.ctx.Err()
		}
	}
	return run.ctx.Err()
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
	defer run.mu.Unlock()

	usefulAt := run.usefulAt
	if usefulAt.IsZero() {
		usefulAt = time.Now()
	}
	return Result{
		Scenario:               run.scenario,
		RTTMS:                  run.options.RTT.Milliseconds(),
		TTFURMS:                usefulAt.Sub(run.started).Milliseconds(),
		BlockingRoundTrips:     run.blockingRoundTrips,
		TotalDBOperations:      run.totalOperations,
		BackgroundCompletionMS: run.background.Milliseconds(),
		RowsConsumed:           run.rowsConsumed,
		RowsRendered:           run.rowsRendered,
		BytesConsumed:          run.bytesConsumed,
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
