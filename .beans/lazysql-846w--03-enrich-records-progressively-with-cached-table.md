---
# lazysql-846w
title: '[03] Enrich Records progressively with cached table metadata'
status: completed
type: task
priority: high
tags:
    - ticket-key-progressive-table-metadata
    - network-performance
    - metadata
    - cache
    - foreign-keys
    - ready-for-agent
    - order-03
    - records
created_at: 2026-09-12T03:19:43Z
updated_at: 2026-09-12T07:20:56Z
parent: lazysql-a9lf
blocked_by:
    - lazysql-rfbf
---

## What to build

After Records have rendered, load uncached Columns, Primary Keys, Foreign Keys, Constraints, and Indexes in background and progressively enrich the visible table as each capability becomes available.

Introduce connection-scoped metadata state/cache shared independently of individual ResultsTable instances. Deduplicate identical in-flight metadata requests and protect UI updates from stale results.

PK/FK functionality must become available independently: FK cells should gain their underline/navigation behavior only after FK metadata arrives, without refetching Records.

## Acceptance criteria

- [x] Records render before any structural metadata load is required to finish.
- [x] Columns, PKs, FKs, constraints, and indexes have independent `unloaded/loading/ready/failed` state or equivalent explicit state.
- [x] An empty successful metadata result is distinguishable from unloaded and failed state.
- [x] Uncached metadata operations may execute concurrently after Records are visible without a priority scheduler.
- [x] Metadata is cached for the lifetime of the database connection using database/schema/table/kind identity.
- [x] Reopening or repaginating the same table reuses valid cached structural metadata.
- [x] Two concurrent requests for the same metadata key cause one underlying database query.
- [x] Completion of metadata for an old table cannot mutate a newer visible table.
- [x] Valid late results may populate the cache even when they no longer apply to the active view.
- [x] PK-dependent capabilities become available when PK metadata arrives without waiting for other metadata.
- [x] FK-jump cells gain underline/navigation behavior when FK metadata arrives without refetching Records.
- [x] Failure of one metadata kind does not remove Records or unrelated successful metadata.
- [x] Regression tests cover empty metadata, failed metadata, cache reuse, in-flight deduplication, and stale-result protection.

## Summary of Changes

- Added a Home-scoped structural metadata cache keyed by database, schema, table, and metadata kind.
- Loaded columns, primary keys, foreign keys, constraints, and indexes concurrently after Records paint, with explicit per-kind states and stale-view guards.
- Reused cached/in-flight column metadata for SQL editor autocomplete and added regression coverage for empty, failed, cached, concurrent, stale, and late FK metadata.
