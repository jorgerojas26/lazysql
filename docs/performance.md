# Network-performance contract

LazySQL measures the network-performance behavior as a contract, not as a
subjective "feels faster" claim. The contract has two layers:

1. deterministic tests assert database-operation and rendering invariants;
2. a local synthetic harness records wall-clock observations at controlled RTTs.

## Run the benchmark matrix

The harness has no database dependency and never needs credentials:

```console
go run ./cmd/lazysql-benchmark
```

The default matrix runs every scenario at `0ms`, `50ms`, and `100ms` artificial
RTT. Use JSON for machine-readable output or select a smaller RTT matrix:

```console
go run ./cmd/lazysql-benchmark -format json
go run ./cmd/lazysql-benchmark -rtt 0ms,50ms
```

Each record reports:

| Field | Meaning |
| --- | --- |
| `ttfur_ms` | Time to the first useful result (TTFUR) |
| `blocking_round_trips` | Database operations required before that result |
| `total_db_operations` | Blocking plus background operations |
| `background_completion_ms` | Time for background enrichment after first paint |
| `rows_consumed` | Rows read, including page/cap lookahead where applicable |
| `rows_rendered` | Rows made visible to the user |
| `bytes_consumed` | Approximate transferred bytes in the synthetic workload |

The harness records timings for comparison. CI gates the deterministic counts,
not a fragile millisecond threshold.

### Scenarios

The matrix covers:

- small Records tables;
- a large table with a slow exact count;
- a large MySQL catalog matching the issue-#340 foreign-key shape;
- pagination, filtered Records, and sorting;
- autocomplete schemas with 100, 500, and 2,000 tables;
- SQL results beyond `max_query_rows`;
- slowly produced streamed rows; and
- full CSV export.

## Local debug logging

Use `--loglevel debug --logfile /path/to/lazysql.jsonl` to write local JSONL
records. Performance records contain an operation, duration, stable database
identity where available, row/byte counts, and outcome fields such as
`cancelled`, `error`, `cache_hit`, or `fallback`.

The first-paint events are explicit:

```json
{"message":"first_useful_result","additional_info":{"event":"first_useful_result","surface":"records","duration":"..."}}
{"message":"first_useful_result","additional_info":{"event":"first_useful_result","surface":"query_editor","duration":"..."}}
```

These are local diagnostics only. SQL text, arguments, row values, credentials,
and connection URLs are not written to performance logs, and LazySQL sends no
telemetry.

## Acceptance matrix

The following matrix is the review checklist for this initiative. Test names are
stable seams; synchronization timeouts in UI tests only prevent a hung test and
are not performance thresholds.

| Contract | CI evidence |
| --- | --- |
| Records first paint uses one page fetch and no count/metadata prerequisite | `components/results_table_test.go`: `TestFetchRecordsRendersBeforeMetadata`, `TestFetchRecordsRendersBeforeSlowForeignKeys` |
| Pagination uses page fetches and cached structural metadata | `components/performance_contract_test.go`: `TestPaginationUsesPageFetchOnlyAfterMetadataIsCached`; `components/refresh_test.go` |
| Count timeout/cancellation and manual cancellation | `components/row_count_test.go`: `TestAutomaticCountTimeoutCancelsDriverWithoutTouchingRecords`, `TestManualExactCountIgnoresAutomaticTimeoutAndTogglesCancellation` |
| Metadata reuse, in-flight deduplication, and stale-result protection | `components/table_metadata_test.go`: `TestMetadataCacheReusesReadyResult`, `TestMetadataCacheDeduplicatesInFlightRequests`, `TestStaleMetadataResultCannotUpdateNewerView`, `TestStaleMetadataIdentityCannotUpdateRevisitedTable` |
| SQL cap and streaming cancellation | `components/sql_editor_pipeline_test.go`, `components/sql_editor_stream_test.go`, `drivers/query_stream_test.go` |
| Export visibility, cap bypass, cancellation, replay safety, and atomicity | `components/csv_export_test.go`, `helpers/csv_test.go` |
| Context-aware driver cancellation and row caps | `drivers/row_count_test.go`, `drivers/sqlite_test.go`, `drivers/query_stream_test.go` |
| Pool defaults, overrides, validation, and SQLite 1/1 | `app/config_test.go`, `drivers/pool_test.go` |
| RTT/scenario coverage and machine-readable metrics | `internal/benchmarks/harness_test.go` |
| Structured operation and first-useful-result logging | `helpers/logger/logger_test.go` plus the operation seams in `components/` |
| User-facing result/count/cancellation/export/config/keymap contract | README sections [Result and network semantics](../README.md#result-and-network-semantics), [Application settings](../README.md#application-settings), and [Default Keybindings](../README.md#default-keybindings) |

The pull-request workflow runs the regular suite and a named network-performance
contract step so regressions are visible separately from benchmark observations.
