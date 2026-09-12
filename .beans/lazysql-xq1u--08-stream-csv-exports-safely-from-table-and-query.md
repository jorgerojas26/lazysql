---
# lazysql-xq1u
title: '[08] Stream CSV exports safely from table and query results'
status: todo
type: task
priority: normal
tags:
    - csv
    - export
    - streaming
    - ready-for-agent
    - order-08
    - ticket-key-stream-csv-exports
    - network-performance
created_at: 2026-09-12T03:19:44Z
updated_at: 2026-09-12T03:19:45Z
parent: lazysql-a9lf
blocked_by:
    - lazysql-rfbf
    - lazysql-lone
---

## What to build

Make full CSV export independent of interactive pagination/counts and SQL-editor row caps.

Keep table actions for current-page versus all-record export, and add query-result actions for visible versus all results. Full exports must stream database rows directly to a temporary CSV file, be cancellable, report row-based progress, and commit atomically only when complete.

For SQL-editor resultsets, `Export All Results` may reexecute the original statement only when LazySQL can conservatively classify it as replay-safe/read-only.

## Acceptance criteria

- [ ] Table export continues to offer `Export Current Page` and `Export All Records`.
- [ ] `Export All Records` respects active filters and sorting.
- [ ] `Export All Records` iterates batches until the returned batch proves the end and does not require an exact count.
- [ ] Query result export offers `Export Visible Results` for any resultset already shown.
- [ ] Query result export offers `Export All Results` only for statements conservatively classified as replay-safe/read-only.
- [ ] Unknown or potentially mutating result-producing SQL cannot be automatically reexecuted for Export All.
- [ ] Full export ignores `max_query_rows`.
- [ ] Full export streams DB rows to the CSV writer without materializing the entire result in memory.
- [ ] Export UI clearly explains that Export All may reexecute a query and may take significant time.
- [ ] Export progress works without a known exact total and reports rows written.
- [ ] Export can be cancelled and cancellation reaches the active database operation.
- [ ] Cancellation or failure aborts the temporary file and does not leave a partial file at the requested final path.
- [ ] Successful export commits/renames the temporary file atomically.
- [ ] Tests cover filtered table export, unknown totals, editor visible/all behavior, replay-safety refusal, cap bypass, cancellation, and atomic file handling.
