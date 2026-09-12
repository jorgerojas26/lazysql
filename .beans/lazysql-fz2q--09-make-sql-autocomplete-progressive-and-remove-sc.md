---
# lazysql-fz2q
title: '[09] Make SQL autocomplete progressive and remove schema N+1 queries'
status: completed
type: task
priority: normal
tags:
    - schema-loader
    - metadata
    - ready-for-agent
    - order-09
    - ticket-key-progressive-autocomplete
    - network-performance
    - autocomplete
created_at: 2026-09-12T03:19:44Z
updated_at: 2026-09-12T17:20:04Z
parent: lazysql-a9lf
blocked_by:
    - lazysql-846w
---

## What to build

Replace the current table-by-table sequential column discovery pattern with progressive schema loading backed by the shared connection-scoped metadata loader/cache.

Table names should become available to autocomplete immediately. Column metadata should load efficiently in bulk for normal schemas and lazily for large schemas, while allowing an immediate per-table fetch when the user requests completion such as `users.`.

## Acceptance criteria

- [x] Table names can become available to autocomplete before all table columns are loaded.
- [x] Autocomplete reuses table/column metadata already present in the connection-scoped cache.
- [x] Autocomplete no longer performs an unconditional sequential `GetTableColumns` network call for every table.
- [x] `schema_bulk_load_threshold` is configurable and defaults to 200 tables.
- [x] Schemas at or below the threshold prefer an efficient bulk-column strategy when supported by the driver.
- [x] Schemas above the threshold load columns lazily rather than bulk-fetching every table.
- [x] `schema_bulk_load_threshold = 0` means always lazy.
- [x] When completion for a specific uncached table is requested, that table's columns are fetched immediately.
- [x] Hidden/excluded schemas are not loaded merely for autocomplete.
- [x] Driver-specific bulk schema capabilities remain behind the shared loader rather than ResultsTable/UI heuristics.
- [x] Tests cover small bulk schemas, large lazy schemas, on-demand table completion, hidden schemas, and cache reuse.

## Summary of Changes

- Added a connection-scoped schema loader that caches table lists and column metadata, filters hidden schemas, and progressively publishes autocomplete data.
- Added optional efficient bulk column loading for supported drivers with lazy and on-demand fallbacks.
- Added the configurable `schema_bulk_load_threshold` setting (default 200; zero means always lazy), editor completion callbacks, and focused tests/documentation.
