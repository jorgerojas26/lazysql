---
# lazysql-lone
title: '[07] Stream SQL-editor resultsets with cancellation and a safety cap'
status: completed
type: task
priority: high
tags:
    - cancellation
    - ready-for-agent
    - order-07
    - ticket-key-stream-sql-results
    - network-performance
    - sql-editor
    - streaming
created_at: 2026-09-12T03:19:44Z
updated_at: 2026-09-12T10:30:08Z
parent: lazysql-a9lf
---

## What to build

Make SQL-editor resultsets appear incrementally instead of materializing the entire result before rendering.

Stream database rows in bounded batches, render the first useful batch promptly, enforce a configurable interactive result cap of 1,000 rows by default, support contextual `Esc` cancellation, and preserve already received rows when a query is cancelled or fails after partial delivery.

## Acceptance criteria

- [x] A result-producing SQL query can render its first batch before the entire resultset has been consumed.
- [x] Streaming is implemented behind a driver-facing contract that does not depend on tview.
- [x] Batches flush using an internal bounded policy equivalent to roughly 100 rows or 50 ms, whichever comes first.
- [x] `max_query_rows` is configurable and defaults to 1000.
- [x] With a finite cap, LazySQL consumes no more than `max_query_rows + 1` rows to detect truncation.
- [x] The lookahead row is not displayed.
- [x] Truncated results clearly state that only the configured maximum rows were shown.
- [x] `max_query_rows = 0` allows unlimited interactive results.
- [x] While a result query is active, contextual `Esc` cancels it without changing normal editor Escape behavior when no result query is active.
- [x] Cancellation reaches the active database operation through context.
- [x] Rows already rendered remain visible after cancellation and are labelled as partial/cancelled.
- [x] Rows already rendered remain visible after a later stream/database error and are labelled as partial/error.
- [x] Query history records SQL that was actually dispatched, including completed, truncated, cancelled, and post-dispatch failed queries.
- [x] SQL rejected before dispatch by client-side validation is not added to history.
- [x] Tests cover first-batch rendering, cap/truncation, cancellation, partial failures, and history semantics.

## Summary of Changes

- Added a tview-free `QueryStreamer` contract with context-aware database row streaming, 100-row/50-ms batching, bounded lookahead truncation, and unlimited mode.
- Added `max_query_rows` configuration with a 1000-row default and visible truncation/partial-result status.
- Added progressive SQL-editor rendering, contextual `CancelQuery`/`Esc`, context cancellation, partial-result preservation, and dispatch-accurate history recording.
- Added driver, UI, cancellation, truncation, failure, and history regression tests.
