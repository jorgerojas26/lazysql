---
# lazysql-a9lf
title: network performance & progressive loading
status: draft
type: feature
priority: normal
created_at: 2026-09-12T02:44:42Z
updated_at: 2026-09-12T03:19:40Z
---

# LazySQL Network Performance & Progressive Loading

## Status

Proposed.

This document defines the target architecture and user experience for improving LazySQL's database/network performance across table browsing, metadata loading, pagination, row counts, SQL query execution, CSV export, autocomplete, and tree loading.

The work may be implemented incrementally across multiple pull requests, but all phases in this spec belong to the same performance initiative and the initiative is not considered complete until all phases are implemented.

---

# 1. Problem

LazySQL currently performs more database work than is necessary before showing useful information to the user.

The most visible example is opening a table. The current flow effectively performs:

```text
fetch records
→ exact COUNT(*)
→ columns
→ constraints
→ foreign keys
→ indexes
→ primary keys
→ render records
```

This works reasonably well against small local databases, but becomes expensive when databases are remote or metadata operations are slow.

Issue #340 demonstrates an extreme case on MySQL:

- 106 databases
- 7,249 tables
- ~45 ms network RTT
- foreign-key metadata lookup: ~6.3 seconds
- exact `COUNT(*)` on a 44M-row table: ~32 seconds

The visible records themselves can already have been fetched while LazySQL is still waiting for information that is not required to display them.

This is not only a MySQL-specific problem. Several architectural patterns amplify network latency across all drivers:

- multiple sequential round-trips before rendering;
- exact counts on every table load;
- metadata refetched during unrelated operations;
- schema/autocomplete N+1 queries;
- SQL editor results fully materialized before rendering;
- cancellation that does not reach `database/sql`;
- speculative refreshes after arbitrary SQL;
- secondary metadata blocking primary information.

The main goal of this project is to remove unnecessary database operations from critical user-facing paths.

---

# 2. Primary Goal

Optimize for:

> **Time To First Useful Result (TTFUR)**

For table browsing, "first useful result" means that the requested records are visible and navigable.

For SQL queries, it means that the first useful batch of result rows is visible.

For the tree and autocomplete, it means that partial but useful information is available without waiting for the full schema to load.

The system should prefer progressive enrichment over blocking completeness.

---

# 3. Core Performance Invariant

Opening or paginating a table must require:

> **One blocking database operation in the happy path before records can be rendered.**

That operation is the records query itself.

PKs, FKs, constraints, indexes, row counts, schema information, and other enrichment must not delay the first render.

With a remote database, TTFUR should therefore approach:

```text
one RTT
+ database execution time
+ transfer of the visible page
```

rather than:

```text
N × RTT
+ multiple unrelated database operations
```

No strict wall-clock millisecond target is required because server execution time varies.

---

# 4. Scope

This initiative covers:

- MySQL
- PostgreSQL
- MSSQL
- SQLite
- table Records view
- pagination
- filtering and sorting
- row counts
- table metadata
- PK/FK-based behavior
- SQL editor execution
- query cancellation
- CSV export
- SQL autocomplete
- database tree loading
- schema caching
- connection pooling
- performance logging
- benchmarks
- README documentation

---

# 5. Non-goals

The following are intentionally out of scope:

- keyset/cursor pagination;
- changing Records pagination away from OFFSET;
- server-side truncation or lazy loading of individual large BLOB/TEXT/JSON cells;
- building a complete cross-dialect SQL parser;
- calculating exact counts for arbitrary SQL-editor queries;
- adding a global "refresh absolutely everything" action;
- remote telemetry or analytics;
- changing the behavior of unsupported FK-jump providers;
- changing composite-FK navigation semantics;
- adding connection lifetime tuning such as `conn_max_lifetime`;
- guaranteeing that every database engine can provide a cheap row estimate.

These can be separate follow-up projects.

---

# 6. Design Principles

1. **Records first.**
   If record rows are available, render them immediately.

2. **Background does not mean free.**
   Queries moved out of the critical path must still be efficient.

3. **Never require an exact count to paginate.**

4. **Structural metadata should be reused.**
   Pagination and filtering must not repeatedly load metadata that has not changed.

5. **Cancellation must reach the database.**
   Ignoring stale results is not enough.

6. **Partial useful information beats blocked complete information.**

7. **Do not perform speculative network work when LazySQL does not know that it is necessary.**

8. **Failures in secondary metadata must not invalidate valid records.**

9. **All background UI updates must be protected against stale-result races.**

---

# 7. Records Loading

## 7.1 Page fetch

Records loading becomes an isolated operation.

Conceptually:

```go
type PageResult struct {
    Rows        [][]string
    Query       string
    HasNextPage bool
}
```

The exact Go representation is implementation-defined, but the contract is mandatory:

- it contains the visible page;
- it indicates whether a next page exists;
- it does not perform row-count lookup;
- it does not perform schema-metadata lookup.

The driver fetches:

```text
pageSize + 1
```

rows.

Only `pageSize` rows are displayed.

If an additional row exists:

```text
HasNextPage = true
```

Otherwise:

```text
HasNextPage = false
```

The extra row is not rendered.

---

## 7.2 Offset pagination

This project keeps the existing OFFSET pagination model.

Conceptually:

```sql
SELECT ...
FROM ...
WHERE ...
ORDER BY ...
LIMIT pageSize + 1 OFFSET offset
```

Keyset pagination may be implemented in a later project.

---

# 8. Pagination State

Pagination must stop depending on an exact `TotalRecords`.

The state should conceptually contain:

```go
Offset
Limit
HasNextPage
ExactTotal       optional
EstimatedTotal   optional
```

Navigation rules:

```text
HasPreviousPage = offset > 0
HasNextPage     = result.HasNextPage
```

An exact count or estimate is display information, not pagination control.

---

# 9. Row Count Strategy

Exact `COUNT(*)` must never block the first Records render.

## 9.1 Count states

The UI recognizes three count states:

### Exact

```text
1–300 of 843 rows
```

### Estimated

```text
1–300 of ~4.3M rows
```

### Unknown, with more rows available

```text
1–300+
```

The `~` prefix must only be used for estimates.

---

## 9.2 Free exact counts

When pagination itself proves the end of the resultset, no count query is necessary.

For example:

```text
offset = 0
pageSize = 300
returned rows = 127
HasNextPage = false
```

means:

```text
exact total = 127
```

Likewise:

```text
offset = 900
returned rows = 57
HasNextPage = false
```

means:

```text
exact total = 957
```

This exact total is obtained with zero additional database queries.

---

## 9.3 Cheap estimates

Drivers should expose a separate cheap-estimate operation:

```go
GetEstimatedRowCount(ctx, database, table) (*int64, error)
```

`nil` means that no useful cheap estimate is available.

The absence of an estimate is not an error.

Driver implementations should use inexpensive native catalog/statistics mechanisms where available.

The estimate operation applies only to the unfiltered table cardinality.

A table-level estimate must never be presented as an estimate for an arbitrary filtered result.

---

## 9.4 Automatic exact count

Default configuration:

```toml
exact_count_threshold = 50000
exact_count_timeout_ms = 200
```

For an unfiltered Records view:

```text
records rendered
→ cheap estimate requested in background
```

If:

```text
estimate <= exact_count_threshold
```

LazySQL attempts an exact `COUNT(*)` in background.

The automatic exact count receives a timeout of:

```text
exact_count_timeout_ms
```

If it completes, the UI upgrades from estimated/unknown to exact.

If it times out, LazySQL keeps the estimate.

If no cheap estimate exists, LazySQL may attempt the automatic exact count directly using the same timeout.

The automatic count must never show an intrusive error.

---

## 9.5 Filtered counts

A table-level estimate is not used for:

```sql
WHERE ...
```

Filtered Records behave as:

```text
render page immediately
→ attempt COUNT(*) WHERE ... in background
→ cancel if exact_count_timeout_ms expires
```

Success:

```text
1–300 of 847 rows
```

Timeout/failure:

```text
1–300+
```

If pagination itself proves the last page, the exact total is inferred without running the count.

---

## 9.6 Manual exact count

Add command:

```text
ExactCount
```

Default key:

```text
#
```

When the current count is estimated or unknown, pagination should expose an action hint such as:

```text
1–300 of ~4.3M rows  [# exact]
```

or:

```text
1–300+  [# exact]
```

A manually requested count:

- has no automatic 200 ms timeout;
- remains cancellable through `context.Context`;
- is cancelled when the user leaves the relevant view/table;
- can be cancelled by pressing `#` again.

While active:

```text
Counting… [# cancel]
```

On non-cancellation failure:

```text
Count failed: <reason>  [# retry]
```

This error is non-modal and Records remain usable.

`ExactCount` applies to Records browsing, not arbitrary SQL-editor queries.

---

# 10. Count Caching and Invalidation

Table estimates may be cached for the connection session.

Exact counts belong to the current Records query identity:

```text
table + filter
```

Changing sort does not invalidate the count because sort does not change cardinality.

Changing filter invalidates the exact count.

Pagination may reuse the count.

Refreshing Records invalidates its count state and reruns the normal count strategy.

---

# 11. Progressive Table Metadata

Once Records are rendered, LazySQL starts loading uncached metadata in background.

The following may start concurrently:

```text
columns
primary keys
foreign keys
constraints
indexes
```

No priority scheduler is required.

The implementation should favor simplicity: independent background operations may run concurrently and update the relevant state as they complete.

---

# 12. PK/FK Progressive Enrichment

PK and FK information is functionally important but does not belong in TTFUR.

Before metadata arrives:

- Records are fully visible;
- navigation works normally;
- metadata-dependent actions are not available yet.

When PK information arrives:

- PK-dependent functionality becomes available.

When FK information arrives:

- supported FK-jump cells are enriched in-place;
- navigable FK cells receive their underline styling;
- `Enter` navigation becomes available.

Rows must not be refetched just to add FK styling.

Each capability activates independently.

LazySQL must not wait for all metadata before enabling one completed capability.

---

# 13. Metadata State

Metadata loading state must be explicit.

Conceptually:

```go
type LoadState int

const (
    Unloaded LoadState = iota
    Loading
    Ready
    Failed
)
```

Each metadata type maintains its own state:

```text
Columns
PrimaryKeys
ForeignKeys
Constraints
Indexes
```

An empty successful result is distinct from an unloaded or failed result.

Examples:

```text
Ready + empty PK list
    = table genuinely has no primary key

Failed
    = LazySQL could not determine the primary key
```

This distinction must be reflected in the UI where relevant.

---

# 14. Metadata Cache

Introduce a connection-scoped schema/metadata loader and cache.

The cache must live outside both:

- individual `ResultsTable` instances;
- individual driver implementations.

It is shared by:

```text
Records
Tree
SQL autocomplete
```

A conceptual cache key is:

```text
connection
+ database
+ schema
+ table
+ metadata kind
```

Metadata remains cached for the lifetime of the connection.

No TTL is required.

No LRU limit is required initially.

The cache is destroyed when the connection closes.

---

# 15. Request Deduplication

The metadata loader must deduplicate concurrent identical requests.

If:

```text
users / ForeignKeys
```

is already loading and another consumer requests the same information, the second consumer waits for/reuses the in-flight load rather than starting another database query.

This is particularly important for slow catalog operations.

---

# 16. Stale Result Protection

Cancellation alone is not enough to protect the UI.

Every asynchronous update must verify that it still belongs to the currently displayed identity.

Example race:

```text
open users
→ users FK request starts

open orders
→ orders records render

users FK request finishes
```

The completed `users` result may enter the cache, but it must never update the visible `orders` table.

Use identity/generation checks in addition to context cancellation.

---

# 17. Context-aware Driver API

All driver methods that perform database I/O must accept `context.Context`.

Examples include:

```text
Connect / TestConnection
GetDatabases
GetTables
GetTableColumns
GetConstraints
GetForeignKeys
GetIndexes
FetchPage/GetRecords
GetPrimaryKeyColumnNames
GetEstimatedRowCount
GetExactRowCount
ExecuteQuery / StreamQuery
ExecuteDMLStatement
ExecutePendingChanges
UpdateRecord
DeleteRecord
GetFunctions
GetProcedures
GetViews
object-definition queries
```

Formatting helpers do not require a context.

Internally drivers must use context-aware database APIs:

```go
QueryContext
QueryRowContext
ExecContext
BeginTx
Tx.ExecContext
```

The current non-context I/O methods should be replaced rather than maintained as parallel legacy variants.

This is an internal breaking refactor and does not require backward compatibility.

---

# 18. Refresh Semantics

`R` becomes context-sensitive.

## Records

Refresh only:

```text
current Records page
row-count information
```

Preserve current filter, sort, and pagination semantics.

Do not invalidate already valid:

```text
columns
foreign keys
constraints
indexes
primary keys
```

If PK metadata is missing or previously failed, Records refresh may retry it.

## Columns

Invalidate and reload only Columns.

## Constraints

Invalidate and reload only Constraints.

## Foreign Keys

Invalidate and reload only Foreign Keys.

## Indexes

Invalidate and reload only Indexes.

A global "refresh all metadata" command is not part of this project.

---

# 19. Metadata Failure Behavior

Secondary metadata failure must never invalidate successfully loaded Records.

Example:

```text
Records      Ready
Columns      Ready
PrimaryKeys  Ready
ForeignKeys  Failed
Indexes      Ready
```

Records remain usable.

Opening a failed metadata tab should expose the error non-destructively.

`R` on that tab explicitly retries it.

No automatic backoff retry loop is required.

Background failures must be logged.

---

# 20. MySQL Foreign Key Performance / Issue #340

Moving FKs out of the critical path fixes TTFUR but does not make an inefficient 6-second query acceptable.

MySQL must receive a driver-specific FK lookup optimization.

Preferred strategy:

```text
MySQL 8-compatible InnoDB metadata fast path
or
MySQL 5.6/5.7 InnoDB dictionary fast path
```

with fallback to:

```text
information_schema.KEY_COLUMN_USAGE
```

when the fast path is unavailable, including cases such as:

- permissions;
- server/version compatibility;
- MariaDB differences;
- non-InnoDB behavior.

The lookup must remain cancellable.

Fallback usage and duration should be debug-logged.

Existing FK semantics must be preserved.

This project does not require expanding FK-jump support to drivers where it is currently unsupported.

---

# 21. SQL Editor Streaming

The SQL editor must stop requiring the complete resultset before displaying rows.

Introduce a streaming/batched driver contract.

A callback-style API is preferred conceptually:

```go
StreamQuery(
    ctx,
    query,
    maxRows,
    onBatch,
)
```

The exact signature may differ, but these semantics are required:

- driver knows nothing about `tview`;
- driver reads database rows incrementally;
- results are emitted in batches;
- consumer backpressure is respected;
- errors and cancellation propagate correctly.

---

# 22. Streaming Batch Policy

Use a dual flush condition internally.

Initial target:

```text
100 rows
OR
~50 ms
```

whichever occurs first.

These are internal tuning values, not public configuration.

They may be adjusted based on benchmarks.

The important behavior is that a slowly returning query can display a partial useful batch without waiting to accumulate 100 rows.

---

# 23. SQL Editor Safety Cap

Add configuration:

```toml
max_query_rows = 1000
```

This limits rows consumed/displayed for an interactive SQL-editor resultset.

To detect truncation, LazySQL may read at most:

```text
max_query_rows + 1
```

rows.

The extra row is not shown.

If it exists:

```text
Truncated = true
```

and the query stream is closed/cancelled.

Example status:

```text
1,000 rows shown — result truncated
```

`max_query_rows = 0` means unlimited interactive results.

---

# 24. Query Cancellation

A query running or streaming in the result surface can be cancelled manually.

Add a contextual command:

```text
CancelQuery
```

Default key while an active query result is running:

```text
Esc
```

Example:

```text
Running query… [Esc cancel]
```

This must not replace the normal meaning of `Esc` while editing SQL.

Starting another operation also cancels obsolete active work.

Cancellation must propagate to the driver and database.

---

# 25. Partial SQL Results

If rows have already been rendered and the query later fails or is cancelled, preserve those rows.

Examples:

```text
400 rows — query cancelled
```

```text
400 rows — partial result: connection lost
```

Partial rows must never be presented as a complete result.

---

# 26. Query History

History represents SQL that was actually dispatched to the server.

Save:

```text
completed query
completed but truncated query
cancelled query
query that failed after dispatch
```

Do not save a query rejected before dispatch, such as client-side read-only validation failure.

---

# 27. SQL Result Export

Interactive query limits must not limit explicit exports.

For a resultset, offer:

```text
Export Visible Results
Export All Results
```

## Export Visible Results

Exports exactly the rows already present in the UI.

It is available for any resultset.

## Export All Results

Reexecutes the original query and streams the complete result directly:

```text
database → CSV writer
```

It ignores `max_query_rows`.

It must not materialize the entire export in memory.

It must be cancellable.

Because reexecution can be unsafe, `Export All Results` is only available when LazySQL can conservatively classify the original statement as replay-safe/read-only.

Unknown or potentially mutating statements receive only:

```text
Export Visible Results
```

False negatives are acceptable.

False positives that could repeat side effects are not.

A complete SQL AST parser is not required.

Existing read-only/mutation classification may be extended for this purpose.

---

# 28. Export UI

The export dialog must clearly explain potentially expensive behavior.

For example:

```text
Export All Results re-executes the query and exports the complete result.
Large exports may take significant time.
```

Selecting `Export All` is itself sufficient confirmation.

No additional confirmation modal is required.

During export, when no exact total exists, progress is row-based:

```text
Exporting… 240,000 rows
Esc: Cancel
```

If an exact total happens to be available, a percentage may optionally be displayed.

---

# 29. Table Export

Keep both existing actions:

```text
Export Current Page
Export All Records
```

`Export All Records` must:

- respect current filter;
- respect current sort;
- fetch sequential batches;
- continue until a batch proves the end;
- not require an exact count;
- stream directly to CSV;
- remain cancellable.

---

# 30. Export Atomicity

CSV export must preserve the existing temporary-file semantics:

```text
write temp file
→ Commit()
→ rename to final filename
```

On failure or cancellation:

```text
Abort()
→ delete temp file
```

A failed/cancelled export must not leave a partial file at the requested final path.

---

# 31. SQL Autocomplete

Autocomplete must use progressive schema loading.

Flow:

```text
load table list
→ table names immediately available for autocomplete
→ load columns afterward
```

The editor must not wait for all table columns before becoming useful.

---

# 32. Bulk vs Lazy Autocomplete Columns

Add:

```toml
schema_bulk_load_threshold = 200
```

If the visible schema contains no more than the threshold:

```text
prefer driver-efficient bulk column loading
```

If it exceeds the threshold:

```text
load columns lazily per table
```

When a user requests completion such as:

```text
users.
```

and `users` columns are not cached, request those columns immediately.

`schema_bulk_load_threshold = 0` means always lazy.

The shared schema loader should reuse columns already obtained by Records or another feature.

Hidden/excluded schemas must not be loaded merely for autocomplete.

---

# 33. Driver Support for Bulk Schema Metadata

Bulk schema loading strategy belongs to the shared schema loader.

Drivers provide optimized primitives/capabilities.

A driver capable of efficiently loading many table columns in one catalog query should expose that ability.

Drivers without a useful bulk strategy may fall back to lazy table-level calls.

The `ResultsTable` component must not contain cross-driver networking heuristics.

---

# 34. Progressive Tree Loading

Tree loading follows the same progressive principle.

Target behavior:

```text
GetDatabases
→ render database nodes

for each database:
    GetTables
    → render tables

    GetFunctions
    GetProcedures
    GetViews
    → enrich tree afterward
```

Programming objects must not delay tables that are already known.

Tree data should use the same connection-scoped schema loader/cache as autocomplete and Records.

---

# 35. Schema Filters

Existing schema filtering for PostgreSQL/MSSQL must apply consistently to:

```text
tree
autocomplete
bulk schema loading
lazy schema loading
```

LazySQL must not generate catalog traffic for schemas intentionally hidden by configuration.

---

# 36. DDL Invalidation

After successful arbitrary DDL from the SQL editor:

```text
invalidate the entire schema metadata cache for the connection
```

Do not attempt to parse arbitrary DDL to precisely determine every affected object.

After invalidation:

```text
refresh/rebuild the visible tree progressively in background
```

The SQL result itself is shown first.

Tree rebuilding must not block DDL completion feedback.

---

# 37. DML Refresh Behavior

## Arbitrary SQL editor DML

Do not automatically refresh the currently selected table merely because the editor was opened from that table.

LazySQL does not reliably know which table arbitrary SQL mutated.

Avoid speculative network traffic.

The user may press `R` when needed.

## DML originating from Records

When LazySQL knows the exact modified table:

```text
commit mutation
→ refresh Records
→ refresh row-count information
→ preserve structural metadata cache
```

For simplicity, row-count information may be refreshed after UPDATE as well as INSERT/DELETE.

---

# 38. Connection Pooling

For remote/server databases, defaults are:

```toml
max_open_connections = 8
max_idle_connections = 8
```

These application-level defaults may be overridden per connection.

Example:

```toml
[application]
max_open_connections = 8
max_idle_connections = 8

[[database]]
Name = "Production"
max_open_connections = 3
max_idle_connections = 3
```

If a per-connection value is absent, inherit the application value.

For these two settings:

```text
0 = use LazySQL default
```

It must not mean unlimited.

Configuration where:

```text
max_idle_connections > max_open_connections
```

is invalid and should produce a configuration error rather than being silently corrected.

---

# 39. SQLite Pooling

SQLite is an explicit exception.

Use:

```text
MaxOpenConns = 1
MaxIdleConns = 1
```

regardless of remote-driver defaults.

This prevents multi-connection SQLite semantic issues, especially with in-memory databases.

Background loads may still be launched using the same high-level architecture; `database/sql` will serialize them through the single SQLite connection.

---

# 40. Configuration

Add application settings:

```toml
[application]
max_query_rows = 1000
exact_count_threshold = 50000
exact_count_timeout_ms = 200
schema_bulk_load_threshold = 200
max_open_connections = 8
max_idle_connections = 8
```

Add per-connection overrides:

```toml
[[database]]
max_open_connections = 0
max_idle_connections = 0
```

where zero inherits/uses the LazySQL default according to the rules above.

---

# 41. Configuration Zero Semantics

## `max_query_rows = 0`

Unlimited SQL-editor result rows.

## `exact_count_threshold = 0`

Do not trigger an automatic exact count merely because an estimate is small.

Other automatic-count paths, such as a no-estimate or filtered query, may still use the configured time budget.

## `exact_count_timeout_ms = 0`

Disable automatic exact counts entirely.

This must **not** mean infinite timeout.

Manual `# ExactCount` remains available.

## `schema_bulk_load_threshold = 0`

Always use lazy autocomplete-column loading.

## pool setting = 0

Use LazySQL's configured/default value rather than unlimited connections.

---

# 42. Local Performance Logging

Add structured debug timing around relevant database operations.

Examples:

```text
operation=fetch_records
duration=84ms
rows=301
```

```text
operation=get_foreign_keys
duration=6.3s
driver=mysql
fallback=key_column_usage
```

```text
operation=get_exact_count
duration=201ms
cancelled=true
reason=automatic_timeout
```

Recommended fields where relevant:

```text
driver
database
schema
table
operation
duration
rows
cache_hit
cancelled
error
```

Sensitive values and credentials must never be logged.

---

# 43. TTFUR Logging

The main metric should also be observable in local debug logs.

Records:

```text
event=first_useful_result
surface=records
duration=92ms
```

SQL editor:

```text
event=first_useful_result
surface=query_editor
duration=137ms
rows_in_first_batch=...
```

This is local debugging information only.

No telemetry is sent externally.

---

# 44. README Documentation

README documentation must cover:

### Pagination/count semantics

```text
843 rows   = exact
~4.3M rows = estimated
300+       = exact total unknown, more rows available
```

### Exact count

```text
# = calculate/cancel exact Records count
```

### Automatic count strategy

Explain:

```text
estimate
50k default threshold
200 ms automatic exact-count budget
```

### SQL editor cap

Explain:

```text
max_query_rows = 1000
```

and truncation behavior.

### Query cancellation

Document contextual `Esc` cancellation.

### Export

Document:

```text
Export Visible Results
Export All Results
```

including replay/reexecution behavior.

### New configuration

Document all settings and their zero semantics.

### Connection pool overrides

Document application defaults, per-connection overrides, and SQLite behavior.

---

# 45. Keymap Changes

Add configurable command:

```text
ExactCount → #
```

Add contextual query cancellation behavior:

```text
CancelQuery → Esc
```

without replacing the existing SQL-editor editing/unfocus semantics when no result query is actively running.

Both should appear in Help where applicable.

---

# 46. Errors and UX Rules

Secondary/background errors should not normally open global modal dialogs.

Use local state whenever possible.

Examples:

```text
Foreign Keys unavailable — press R to retry
```

```text
Count failed: permission denied [# retry]
```

```text
400 rows — partial result: connection lost
```

Errors in the primary records/page fetch may continue to use the standard table error flow.

---

# 47. Testing Strategy

Wall-clock timing assertions should not be the main CI gate.

CI should validate deterministic architectural invariants.

Required tests include:

### Records first paint

Opening Records performs exactly one required blocking page-fetch operation before records may be rendered.

No metadata or count operation is required for first paint.

### Pagination

Next/previous page requires one page fetch.

Cached structural metadata is not fetched again.

### Filtering and sorting

Filtering/sorting does not reload cached structural metadata.

Sort does not invalidate row count.

Filter invalidates row-count state.

### `pageSize + 1`

The lookahead row correctly sets `HasNextPage` and is not rendered.

### Exact total inference

A page without a next row derives:

```text
offset + visibleRows
```

as exact total.

### Count timeout

Automatic exact count is cancelled at the configured budget without failing Records.

### Manual count

Manual exact count ignores the automatic timeout but remains cancellable.

### Metadata cache

Repeated requests return cached data without additional DB calls.

### In-flight deduplication

Two identical concurrent metadata requests produce one underlying DB query.

### Metadata failure

Failure does not remove valid Records or unrelated metadata.

### Cancellation

Starting an obsolete operation causes the old DB context to be cancelled.

### Stale update prevention

Completion of an old request cannot modify a newer table view.

### SQL streaming

The first batch can be rendered before the complete result.

### SQL safety cap

Interactive SQL consumes no more than:

```text
max_query_rows + 1
```

rows when capped.

### Partial results

Cancellation/error after batches preserves rows and marks them incomplete.

### Export

Visible export writes visible rows.

All export bypasses `max_query_rows`.

Cancellation/error calls Abort and does not leave a final partial CSV.

### Replay safety

Potentially mutating/unknown result-producing SQL cannot automatically use Export All.

### DDL

Successful DDL invalidates schema cache.

### DML

Arbitrary editor DML does not speculatively refresh the selected table.

Known table DML refreshes Records/count state.

### SQLite

Pool remains 1/1.

### Config validation

Invalid pool combinations are rejected.

Zero-value semantics behave as documented.

---

# 48. Performance Benchmark Matrix

Create a benchmark/integration harness that can simulate artificial RTT.

Test at minimum:

```text
0 ms RTT
50 ms RTT
100 ms RTT
```

Scenarios:

```text
small table
large table with slow exact COUNT(*)
MySQL #340-like large catalog
pagination
filtered Records
sorting
100-table autocomplete schema
500-table autocomplete schema
2,000-table autocomplete schema
SQL query returning > max_query_rows
SQL query producing rows slowly
full CSV export
```

Capture:

```text
TTFUR
blocking round-trips
total database operations
background metadata completion time
rows consumed
bytes consumed where practical
```

Wall-clock benchmark numbers should be recorded and compared, but ordinary CI should not fail because a benchmark exceeded a fragile millisecond threshold.

The strongest CI assertions are query/round-trip invariants.

---

# 49. Acceptance Criteria

The initiative is complete only when all of the following are true:

- Opening Records no longer waits for metadata.
- Opening Records no longer waits for `COUNT(*)`.
- Records happy-path first paint has one blocking DB operation.
- Pagination works without an exact row count.
- Exact/estimated/unknown totals have distinct UI semantics.
- Automatic counts are bounded and cancellable.
- Manual exact count works through `#`.
- Structural metadata loads progressively and is cached.
- PK/FK behavior activates after metadata arrives without refetching Records.
- Metadata requests are deduplicated.
- Stale background results cannot corrupt another view.
- All relevant driver I/O is context-aware.
- MySQL #340 receives an optimized FK lookup with fallback.
- SQL-editor resultsets stream incrementally.
- SQL-editor row cap defaults to 1,000.
- Active queries can be cancelled.
- Partial streamed results are preserved and labelled.
- Full export streams independently of the interactive row cap.
- Export cancellation is atomic.
- Autocomplete avoids the current per-table sequential N+1 pattern.
- Tree loading is progressive.
- Tree, autocomplete, and Records share schema metadata.
- DDL invalidates schema cache.
- Arbitrary editor DML no longer causes speculative table refresh.
- Connection pools use the agreed defaults/overrides.
- SQLite uses a single connection.
- Performance operations and TTFUR are observable in debug logs.
- Benchmark coverage exists.
- README/config/keymap documentation is updated.

---

# 50. Implementation Phases

## Phase 1 — Context-aware I/O and page model

Implement:

```text
context-aware Driver I/O
PageResult / HasNextPage
pageSize + 1
pagination independent of TotalRecords
connection pool configuration
SQLite pool exception
```

Remove blocking exact count from page fetch.

This phase should already produce a significant remote-database speedup.

---

## Phase 2 — Progressive metadata and cache

Implement:

```text
connection-scoped metadata loader
metadata LoadState
metadata cache
in-flight request deduplication
stale-generation checks
Records-first render
concurrent post-render metadata loading
context-sensitive Refresh
```

Ensure FK metadata can restyle existing Records without refetching them.

---

## Phase 3 — Row-count strategy

Implement:

```text
cheap estimates
automatic bounded exact counts
filtered bounded counts
exact total inference from pagination
# ExactCount
count cancellation
count UI states
count caching/invalidation
```

---

## Phase 4 — Driver-specific metadata optimization

Implement efficient metadata operations across drivers.

Explicitly fix MySQL #340 using the InnoDB fast path plus fallback.

Add timing/fallback logging.

---

## Phase 5 — SQL editor streaming and export

Implement:

```text
batched streaming
first-batch rendering
max_query_rows
truncation detection
Esc cancellation
partial result states
new query-history semantics
Export Visible
safe Export All
streaming CSV export
export progress/cancellation
```

---

## Phase 6 — Schema loader, autocomplete, and tree

Implement:

```text
shared SchemaLoader
progressive autocomplete
bulk/lazy threshold
schema-filter reuse
progressive tree
tree/autocomplete cache reuse
DDL invalidation and tree rebuild
remove speculative DML refresh
```

---

## Phase 7 — Performance validation and documentation

Implement:

```text
deterministic performance-contract tests
RTT benchmark harness
#340 regression scenario
debug performance logging
TTFUR logging
README documentation
config documentation
keymap documentation
cleanup of obsolete APIs/state
```

---

# 51. Expected Outcome

The visible performance of LazySQL should become dominated by the query the user actually requested rather than by unrelated metadata work.

Opening a normal remote table should conceptually change from:

```text
records
→ count
→ columns
→ constraints
→ FK
→ indexes
→ PK
→ render
```

to:

```text
records
→ render immediately
```

followed independently by:

```text
columns ───────┐
PK ────────────┤
FK ────────────┤
constraints ───┤── progressive enrichment
indexes ───────┤
count info ────┘
```

Pagination should become:

```text
fetch pageSize + 1
→ render
```

The SQL editor should become:

```text
execute
→ first batch
→ render
→ subsequent batches
→ complete / truncated / cancelled
```

The result should be a significant improvement for any remote connection and an especially large improvement for installations where catalog metadata or exact counts are expensive.

The defining rule is:

> **If LazySQL already has useful data, unrelated database work must not prevent the user from seeing or using it.**
