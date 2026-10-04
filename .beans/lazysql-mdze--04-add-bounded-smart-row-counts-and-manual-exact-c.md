---
# lazysql-mdze
title: '[04] Add bounded smart row counts and manual exact count'
status: completed
type: task
priority: high
tags:
    - ready-for-agent
    - order-04
    - ticket-key-smart-row-counts
    - network-performance
    - counts
    - pagination
    - configuration
created_at: 2026-09-12T03:19:44Z
updated_at: 2026-09-12T07:51:42Z
parent: lazysql-a9lf
blocked_by:
    - lazysql-rfbf
---

## What to build

Restore useful row-total information without putting exact `COUNT(*)` back into the Records critical path.

Support exact totals inferred for free from pagination, cheap driver estimates for unfiltered tables, bounded automatic exact counts, bounded filtered counts, and an explicit `# ExactCount` action for users who require an exact total.

Use default configuration of 50,000 rows for the estimate threshold and 200 ms for automatic exact-count attempts.

## Acceptance criteria

- [x] Pagination works correctly without any exact total.
- [x] If a page proves the end of the resultset, LazySQL derives the exact total as `offset + visibleRows` without another query.
- [x] Drivers can request a cheap table-row estimate independently from exact counting.
- [x] Absence of a cheap estimate is represented as unavailable/unknown rather than an error.
- [x] Unfiltered table estimates at or below `exact_count_threshold` trigger an automatic exact count in background.
- [x] Automatic exact counts use `exact_count_timeout_ms` and cancellation reaches the database.
- [x] When no estimate exists, LazySQL may attempt an automatic exact count using the same bounded timeout.
- [x] Filtered Records never display an unfiltered table estimate as if it described the filtered result.
- [x] Filtered Records may attempt an exact filtered count in background within the configured timeout.
- [x] Count timeout/failure never blocks or invalidates Records.
- [x] UI distinguishes exact totals, approximate totals with `~`, and unknown totals with a `+` form.
- [x] `#` starts a manual exact count for the current Records result when useful.
- [x] Manual exact count is not limited by the automatic 200 ms timeout.
- [x] Pressing `#` again cancels an active manual exact count.
- [x] Manual count failure is shown non-destructively and can be retried.
- [x] Count state is reusable across pagination/sort, invalidated by filter changes, and refreshed by Records Refresh.
- [x] `exact_count_threshold` defaults to 50000 and 0 disables estimate-triggered automatic exact counts.
- [x] `exact_count_timeout_ms` defaults to 200 and 0 disables automatic exact counts without disabling manual `# ExactCount`.
- [x] Count behavior has deterministic tests for inference, estimate/no-estimate paths, timeout, cancellation, filter changes, and manual count.

## Summary of Changes

- Added exact, estimated, and unknown pagination count states with free last-page inference and non-blocking UI hints.
- Added context-aware cheap-estimate and exact-count operations for all database drivers, with bounded automatic counts and cancellable manual `#` counts.
- Added count identity invalidation/reuse across filtering, sorting, pagination, refresh, and tab changes, plus configuration defaults and deterministic tests.
