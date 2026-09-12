---
# lazysql-fwt5
title: '[05] Make Refresh reload only the active surface'
status: todo
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
updated_at: 2026-09-12T03:19:45Z
parent: lazysql-a9lf
blocked_by:
    - lazysql-846w
    - lazysql-mdze
---

## What to build

Make `R` refresh only the information the user is currently viewing instead of reloading unrelated Records and metadata.

Records Refresh should reload the current Records result and row-count state while preserving valid structural metadata. Metadata tabs should invalidate and reload only their own metadata kind. Failed metadata should be explicitly retryable through the same Refresh action.

## Acceptance criteria

- [ ] `R` in Records refreshes the Records page and its row-count state.
- [ ] Records Refresh preserves the active filter, sort, and intended pagination behavior.
- [ ] Records Refresh does not invalidate valid Columns, FKs, Constraints, Indexes, or PK metadata.
- [ ] Records Refresh may retry PK metadata only when it is missing or previously failed.
- [ ] `R` in Columns invalidates and reloads only Columns.
- [ ] `R` in Foreign Keys invalidates and reloads only Foreign Keys.
- [ ] `R` in Constraints invalidates and reloads only Constraints.
- [ ] `R` in Indexes invalidates and reloads only Indexes.
- [ ] A failed metadata surface exposes its failure non-destructively and `R` retries that surface.
- [ ] Refresh does not perform unrelated speculative database operations.
- [ ] Tests assert exact underlying database calls for each active surface.
