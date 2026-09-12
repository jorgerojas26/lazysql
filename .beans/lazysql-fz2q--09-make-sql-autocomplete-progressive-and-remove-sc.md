---
# lazysql-fz2q
title: '[09] Make SQL autocomplete progressive and remove schema N+1 queries'
status: todo
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
updated_at: 2026-09-12T03:19:45Z
parent: lazysql-a9lf
blocked_by:
    - lazysql-846w
---

## What to build

Replace the current table-by-table sequential column discovery pattern with progressive schema loading backed by the shared connection-scoped metadata loader/cache.

Table names should become available to autocomplete immediately. Column metadata should load efficiently in bulk for normal schemas and lazily for large schemas, while allowing an immediate per-table fetch when the user requests completion such as `users.`.

## Acceptance criteria

- [ ] Table names can become available to autocomplete before all table columns are loaded.
- [ ] Autocomplete reuses table/column metadata already present in the connection-scoped cache.
- [ ] Autocomplete no longer performs an unconditional sequential `GetTableColumns` network call for every table.
- [ ] `schema_bulk_load_threshold` is configurable and defaults to 200 tables.
- [ ] Schemas at or below the threshold prefer an efficient bulk-column strategy when supported by the driver.
- [ ] Schemas above the threshold load columns lazily rather than bulk-fetching every table.
- [ ] `schema_bulk_load_threshold = 0` means always lazy.
- [ ] When completion for a specific uncached table is requested, that table's columns are fetched immediately.
- [ ] Hidden/excluded schemas are not loaded merely for autocomplete.
- [ ] Driver-specific bulk schema capabilities remain behind the shared loader rather than ResultsTable/UI heuristics.
- [ ] Tests cover small bulk schemas, large lazy schemas, on-demand table completion, hidden schemas, and cache reuse.
