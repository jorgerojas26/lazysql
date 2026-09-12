package benchmarks

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func fastOptions() Options {
	return Options{
		PageSize:                3,
		MaxQueryRows:            2,
		SchemaBulkLoadThreshold: 200,
	}
}

func resultFor(t *testing.T, results []Result, scenario Scenario) Result {
	t.Helper()
	for _, result := range results {
		if result.Scenario == scenario {
			return result
		}
	}
	t.Fatalf("scenario %q was not returned", scenario)
	return Result{}
}

func TestScenarioMatrixCoversEveryContractWorkload(t *testing.T) {
	results, err := RunWithOptions(context.Background(), fastOptions())
	if err != nil {
		t.Fatalf("RunWithOptions() error = %v", err)
	}
	if len(results) != len(Scenarios()) {
		t.Fatalf("scenario count = %d, want %d", len(results), len(Scenarios()))
	}

	for _, scenario := range Scenarios() {
		result := resultFor(t, results, scenario)
		if result.BlockingRoundTrips < 1 {
			t.Errorf("%s has no blocking round trip", scenario)
		}
		if result.TotalDBOperations < result.BlockingRoundTrips {
			t.Errorf("%s operations = %d, blocking = %d", scenario, result.TotalDBOperations, result.BlockingRoundTrips)
		}
		if result.TTFURMS < 0 {
			t.Errorf("%s has negative TTFUR %dms", scenario, result.TTFURMS)
		}
	}
}

func TestScenarioOperationCountsCaptureProgressiveContract(t *testing.T) {
	results, err := RunWithOptions(context.Background(), fastOptions())
	if err != nil {
		t.Fatalf("RunWithOptions() error = %v", err)
	}

	small := resultFor(t, results, ScenarioSmallTable)
	if small.BlockingRoundTrips != 1 || small.TotalDBOperations != 6 {
		t.Fatalf("small table operations = blocking %d total %d, want 1/6", small.BlockingRoundTrips, small.TotalDBOperations)
	}

	count := resultFor(t, results, ScenarioLargeSlowCount)
	if count.BlockingRoundTrips != 1 || count.TotalDBOperations != 3 {
		t.Fatalf("slow count operations = blocking %d total %d, want 1/3", count.BlockingRoundTrips, count.TotalDBOperations)
	}

	catalog := resultFor(t, results, ScenarioMySQLCatalog)
	if catalog.BlockingRoundTrips != 1 || catalog.TotalDBOperations != 2 {
		t.Fatalf("MySQL catalog operations = blocking %d total %d, want 1/2", catalog.BlockingRoundTrips, catalog.TotalDBOperations)
	}

	pagination := resultFor(t, results, ScenarioPagination)
	if pagination.BlockingRoundTrips != 2 || pagination.TotalDBOperations != 2 {
		t.Fatalf("pagination operations = blocking %d total %d, want 2/2", pagination.BlockingRoundTrips, pagination.TotalDBOperations)
	}

	for _, scenario := range []Scenario{ScenarioFilteredRecords, ScenarioSorting} {
		result := resultFor(t, results, scenario)
		if result.BlockingRoundTrips != 1 || result.TotalDBOperations != 1 {
			t.Fatalf("%s operations = blocking %d total %d, want 1/1", scenario, result.BlockingRoundTrips, result.TotalDBOperations)
		}
	}

	for _, scenario := range []Scenario{ScenarioAutocomplete100, ScenarioAutocomplete500, ScenarioAutocomplete2000} {
		result := resultFor(t, results, scenario)
		if result.BlockingRoundTrips != 1 || result.TotalDBOperations != 2 {
			t.Fatalf("%s operations = blocking %d total %d, want 1/2", scenario, result.BlockingRoundTrips, result.TotalDBOperations)
		}
	}
}

func TestScenarioRowCapsAndExportMetrics(t *testing.T) {
	results, err := RunWithOptions(context.Background(), fastOptions())
	if err != nil {
		t.Fatalf("RunWithOptions() error = %v", err)
	}

	capped := resultFor(t, results, ScenarioSQLRowCap)
	if capped.RowsConsumed != 3 || capped.RowsRendered != 2 {
		t.Fatalf("capped SQL rows consumed/rendered = %d/%d, want 3/2", capped.RowsConsumed, capped.RowsRendered)
	}

	export := resultFor(t, results, ScenarioFullExport)
	if export.RowsConsumed != 10_000 || export.RowsRendered != 10_000 {
		t.Fatalf("full export rows consumed/rendered = %d/%d, want 10000/10000", export.RowsConsumed, export.RowsRendered)
	}
	if export.BytesConsumed <= 0 {
		t.Fatalf("full export bytes = %d, want a positive byte count", export.BytesConsumed)
	}
}

func TestHarnessSupportsDefaultArtificialRTTs(t *testing.T) {
	for _, rtt := range RTTs() {
		result, err := RunScenario(context.Background(), rtt, ScenarioSmallTable)
		if err != nil {
			t.Fatalf("RunScenario(%s) error = %v", rtt, err)
		}
		if result.RTTMS != rtt.Milliseconds() {
			t.Errorf("reported RTT = %dms, want %dms", result.RTTMS, rtt.Milliseconds())
		}
	}
}

func TestHarnessCancellationIsContextAware(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := RunWithOptions(ctx, fastOptions())
	if err != context.Canceled {
		t.Fatalf("RunWithOptions() error = %v, want context.Canceled", err)
	}
}

func TestBenchmarkOutputContainsContractMetrics(t *testing.T) {
	results, err := RunWithOptions(context.Background(), fastOptions())
	if err != nil {
		t.Fatalf("RunWithOptions() error = %v", err)
	}

	var output bytes.Buffer
	if err := WriteJSON(&output, results); err != nil {
		t.Fatalf("WriteJSON() error = %v", err)
	}
	for _, field := range []string{
		"rtt_ms",
		"ttfur_ms",
		"blocking_round_trips",
		"total_db_operations",
		"background_completion_ms",
		"rows_consumed",
		"bytes_consumed",
	} {
		if !strings.Contains(output.String(), `"`+field+`"`) {
			t.Errorf("JSON output does not contain %q: %s", field, output.String())
		}
	}

	var decoded []Result
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil {
		t.Fatalf("JSON output is invalid: %v", err)
	}
	if len(decoded) != len(results) {
		t.Fatalf("decoded result count = %d, want %d", len(decoded), len(results))
	}
}

func TestParseRTTs(t *testing.T) {
	rtts, err := ParseRTTs("0ms, 50ms, 100ms")
	if err != nil {
		t.Fatalf("ParseRTTs() error = %v", err)
	}
	want := []time.Duration{0, 50 * time.Millisecond, 100 * time.Millisecond}
	if len(rtts) != len(want) {
		t.Fatalf("parsed RTT count = %d, want %d", len(rtts), len(want))
	}
	for i := range want {
		if rtts[i] != want[i] {
			t.Errorf("RTT[%d] = %s, want %s", i, rtts[i], want[i])
		}
	}
}
