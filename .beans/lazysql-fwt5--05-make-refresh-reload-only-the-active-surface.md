---
# lazysql-fwt5
title: '[05] Make Refresh reload only the active surface'
status: completed
type: task
priority: normal
tags:
    - ready-for-agent
    - order-05
    - ticket-key-context-sensitive-refresh
    - network-performance
    - refresh
    - metadata
    - records
created_at: 2026-09-12T03:19:44Z
updated_at: 2026-09-12T08:14:09Z
parent: lazysql-a9lf
blocked_by:
    - lazysql-846w
    - lazysql-mdze
---

## What to build

Make `R` refresh only the information the user is currently viewing instead of reloading unrelated Records and metadata.

Records Refresh should reload the current Records result and row-count state while preserving valid structural metadata. Metadata tabs should invalidate and reload only their own metadata kind. Failed metadata should be explicitly retryable through the same Refresh action.

## Acceptance criteria

- [x] `R` in Records refreshes the Records page and its row-count state.
- [x] Records Refresh preserves the active filter, sort, and intended pagination behavior.
- [x] Records Refresh does not invalidate valid Columns, FKs, Constraints, Indexes, or PK metadata.
- [x] Records Refresh may retry PK metadata only when it is missing or previously failed.
- [x] `R` in Columns invalidates and reloads only Columns.
- [x] `R` in Foreign Keys invalidates and reloads only Foreign Keys.
- [x] `R` in Constraints invalidates and reloads only Constraints.
- [x] `R` in Indexes invalidates and reloads only Indexes.
- [x] A failed metadata surface exposes its failure non-destructively and `R` retries that surface.
- [x] Refresh does not perform unrelated speculative database operations.
- [x] Tests assert exact underlying database calls for each active surface.

## Summary of Changes

- Routed `R` by the active Results surface: Records refreshes the current page/count state, while metadata tabs invalidate and reload only their own kind.
- Limited Records refresh metadata work to missing/failed primary keys and preserved filter, sort, pagination, and valid metadata caches.
- Added safe metadata-cache invalidation for in-flight requests, local retry messaging/logging for failures, and regression tests asserting exact driver calls.
- Verified with `go test ./... -count=1` and `go vet ./components`.
