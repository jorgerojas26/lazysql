---
# lazysql-xcd4
title: '[11] Contract legacy driver I/O into one context-aware API'
status: todo
type: task
priority: normal
tags:
    - ready-for-agent
    - order-11
    - ticket-key-contract-context-aware-driver-api
    - network-performance
    - driver-api
    - refactor
    - cancellation
created_at: 2026-09-12T03:19:45Z
updated_at: 2026-09-12T03:19:46Z
parent: lazysql-a9lf
blocked_by:
    - lazysql-fwt5
    - lazysql-532r
    - lazysql-xq1u
    - lazysql-gw9j
---

## What to build

Finish the expand-contract migration created by the preceding tracer bullets so all remaining database I/O uses a single context-aware Driver contract.

Remove contextless legacy I/O variants that were temporarily retained during migration, convert remaining calls to context-aware database/sql APIs, and leave one coherent cancellation model across all supported drivers, mocks, and tests.

This ticket is intentionally a wide-refactor contract step rather than a new user-facing feature.

## Acceptance criteria

- [ ] All Driver methods that perform database I/O accept or otherwise operate under the caller's `context.Context`.
- [ ] Remaining query operations use context-aware database/sql APIs such as `QueryContext`, `QueryRowContext`, and `ExecContext`.
- [ ] Transactional execution uses a context-aware transaction path such as `BeginTx` and context-aware transaction statements.
- [ ] Obsolete contextless Driver I/O methods introduced/retained for migration are removed.
- [ ] Formatting-only helpers remain context-free.
- [ ] MySQL, PostgreSQL, MSSQL, and SQLite compile against the single final Driver interface.
- [ ] Test mocks/fakes compile against the final Driver interface without parallel legacy methods.
- [ ] Existing cancellation behavior from Records, metadata, counts, streaming queries, and exports remains green after contraction.
- [ ] No caller can accidentally choose a non-cancellable legacy database-I/O path.
