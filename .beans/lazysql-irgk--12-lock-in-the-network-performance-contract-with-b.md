---
# lazysql-irgk
title: '[12] Lock in the network-performance contract with benchmarks, logs, and docs'
status: todo
type: task
priority: normal
tags:
    - order-12
    - ticket-key-performance-contract-observability-docs
    - benchmarks
    - observability
    - documentation
    - ttfur
    - ready-for-agent
    - network-performance
created_at: 2026-09-12T03:19:45Z
updated_at: 2026-09-12T03:19:46Z
parent: lazysql-a9lf
blocked_by:
    - lazysql-fwt5
    - lazysql-532r
    - lazysql-xq1u
    - lazysql-fz2q
    - lazysql-gw9j
    - lazysql-xcd4
---

## What to build

Make the new network-performance behavior measurable, regression-resistant, and understandable to users and maintainers.

Add deterministic CI checks for query/round-trip invariants, an integration/benchmark harness with artificial RTT and issue-#340-style scenarios, structured local debug timings including TTFUR, and complete README/keymap/config documentation for the new behavior.

## Acceptance criteria

- [ ] CI verifies that opening Records requires one blocking page-fetch operation before first render.
- [ ] CI verifies that pagination does not require exact counts or refetch cached structural metadata.
- [ ] CI verifies count timeout/cancellation, metadata cache reuse, in-flight deduplication, stale-result protection, SQL row caps, streaming cancellation, and export atomicity.
- [ ] Performance tests avoid fragile pass/fail wall-clock millisecond assertions where deterministic operation-count assertions are possible.
- [ ] A benchmark/integration harness can exercise at least 0 ms, 50 ms, and 100 ms artificial RTT.
- [ ] Benchmark scenarios include small tables, large/slow counts, an issue-#340-like MySQL catalog, pagination, filtering, autocomplete schemas around 100/500/2000 tables, SQL results beyond the row cap, slowly produced rows, and full export.
- [ ] Benchmark output captures TTFUR, blocking round-trips, total DB operations, background completion time, and rows/bytes where practical.
- [ ] Debug logs record operation name, duration, relevant DB identity, cancellation/failure, and cache/fallback information without logging credentials or sensitive values.
- [ ] Debug logs expose `first_useful_result` timing for Records and SQL-editor resultsets.
- [ ] README documents exact (`843`), estimated (`~4.3M`), and unknown-more (`300+`) row-count semantics.
- [ ] README documents `# ExactCount`, its toggle/cancellation behavior, and automatic-count threshold/timeout behavior.
- [ ] README documents `max_query_rows`, SQL truncation, contextual query cancellation, visible/all export behavior, and replay-safe reexecution.
- [ ] README/config docs cover `schema_bulk_load_threshold`, pool settings/overrides, zero-value semantics, and SQLite's 1/1 pool behavior.
- [ ] Help/keymap documentation includes the new ExactCount and active-query cancellation behavior.
- [ ] The original network-performance initiative can be evaluated against the documented acceptance matrix rather than subjective 'feels faster' claims.
