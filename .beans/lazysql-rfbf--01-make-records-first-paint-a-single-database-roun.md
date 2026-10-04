---
# lazysql-rfbf
title: '[01] Make Records first paint a single database round-trip'
status: completed
type: task
priority: high
tags:
    - order-01
    - ticket-key-records-single-round-trip
    - network-performance
    - records
    - pagination
    - ttfur
    - ready-for-agent
created_at: 2026-09-12T03:19:43Z
updated_at: 2026-09-12T04:28:36Z
parent: lazysql-a9lf
---

## What to build

Make opening, paginating, filtering, and sorting the Records view render rows after a single blocking page-fetch operation. Remove exact row counting and table metadata from the first-paint critical path.

Keep OFFSET pagination, fetch `pageSize + 1` rows to determine whether a next page exists, render only `pageSize`, and make page loading cancellable end-to-end so obsolete navigation work does not continue consuming database resources.

This ticket is the first tracer bullet for the initiative: after it lands, remote table browsing should already feel materially faster even before metadata caching, smart counts, or schema loading are implemented.

## Acceptance criteria

- [x] Opening Records requires exactly one blocking database page-fetch operation before rows can be rendered.
- [x] `COUNT(*)` is not executed as part of the blocking Records page fetch.
- [x] Columns, PKs, FKs, constraints, and indexes are not required before the first Records render.
- [x] Records fetch requests `pageSize + 1` rows, renders at most `pageSize`, and exposes whether a next page exists.
- [x] Previous-page availability is derived from the current offset rather than a total row count.
- [x] Next-page availability is derived from the lookahead row rather than a total row count.
- [x] Filtering and sorting preserve OFFSET pagination and the one-blocking-fetch first-paint invariant.
- [x] Starting a newer Records load cancels the older database operation through `context.Context`.
- [x] A cancelled/stale Records result cannot overwrite a newer visible page.
- [x] Existing supported Records navigation, filtering, and sorting behavior remains functional.
- [x] Tests assert database-operation counts rather than fragile wall-clock thresholds.


## Summary of Changes

- Added a context-aware `PageResult` Records API for all supported drivers.
- Removed exact count queries from page fetches and added `pageSize + 1` lookahead trimming.
- Rendered Records before asynchronous metadata enrichment and made pagination use offset/lookahead state.
- Added generation-based cancellation and stale-result protection for Records loads.
- Added deterministic driver, pagination, first-paint, and cancellation tests.
