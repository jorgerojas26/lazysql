---
# lazysql-rfbf
title: '[01] Make Records first paint a single database round-trip'
status: todo
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
updated_at: 2026-09-12T03:19:43Z
parent: lazysql-a9lf
---

## What to build

Make opening, paginating, filtering, and sorting the Records view render rows after a single blocking page-fetch operation. Remove exact row counting and table metadata from the first-paint critical path.

Keep OFFSET pagination, fetch `pageSize + 1` rows to determine whether a next page exists, render only `pageSize`, and make page loading cancellable end-to-end so obsolete navigation work does not continue consuming database resources.

This ticket is the first tracer bullet for the initiative: after it lands, remote table browsing should already feel materially faster even before metadata caching, smart counts, or schema loading are implemented.

## Acceptance criteria

- [ ] Opening Records requires exactly one blocking database page-fetch operation before rows can be rendered.
- [ ] `COUNT(*)` is not executed as part of the blocking Records page fetch.
- [ ] Columns, PKs, FKs, constraints, and indexes are not required before the first Records render.
- [ ] Records fetch requests `pageSize + 1` rows, renders at most `pageSize`, and exposes whether a next page exists.
- [ ] Previous-page availability is derived from the current offset rather than a total row count.
- [ ] Next-page availability is derived from the lookahead row rather than a total row count.
- [ ] Filtering and sorting preserve OFFSET pagination and the one-blocking-fetch first-paint invariant.
- [ ] Starting a newer Records load cancels the older database operation through `context.Context`.
- [ ] A cancelled/stale Records result cannot overwrite a newer visible page.
- [ ] Existing supported Records navigation, filtering, and sorting behavior remains functional.
- [ ] Tests assert database-operation counts rather than fragile wall-clock thresholds.
