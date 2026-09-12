---
# lazysql-846w
title: '[03] Enrich Records progressively with cached table metadata'
status: todo
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
updated_at: 2026-09-12T03:19:45Z
parent: lazysql-a9lf
blocked_by:
    - lazysql-rfbf
---

## What to build

After Records have rendered, load uncached Columns, Primary Keys, Foreign Keys, Constraints, and Indexes in background and progressively enrich the visible table as each capability becomes available.

Introduce connection-scoped metadata state/cache shared independently of individual ResultsTable instances. Deduplicate identical in-flight metadata requests and protect UI updates from stale results.

PK/FK functionality must become available independently: FK cells should gain their underline/navigation behavior only after FK metadata arrives, without refetching Records.

## Acceptance criteria

- [ ] Records render before any structural metadata load is required to finish.
- [ ] Columns, PKs, FKs, constraints, and indexes have independent `unloaded/loading/ready/failed` state or equivalent explicit state.
- [ ] An empty successful metadata result is distinguishable from unloaded and failed state.
- [ ] Uncached metadata operations may execute concurrently after Records are visible without a priority scheduler.
- [ ] Metadata is cached for the lifetime of the database connection using database/schema/table/kind identity.
- [ ] Reopening or repaginating the same table reuses valid cached structural metadata.
- [ ] Two concurrent requests for the same metadata key cause one underlying database query.
- [ ] Completion of metadata for an old table cannot mutate a newer visible table.
- [ ] Valid late results may populate the cache even when they no longer apply to the active view.
- [ ] PK-dependent capabilities become available when PK metadata arrives without waiting for other metadata.
- [ ] FK-jump cells gain underline/navigation behavior when FK metadata arrives without refetching Records.
- [ ] Failure of one metadata kind does not remove Records or unrelated successful metadata.
- [ ] Regression tests cover empty metadata, failed metadata, cache reuse, in-flight deduplication, and stale-result protection.
