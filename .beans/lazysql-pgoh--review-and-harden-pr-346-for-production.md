---
# lazysql-pgoh
title: Review and harden PR 346 for production
status: completed
type: bug
priority: high
created_at: 2026-09-22T06:03:36Z
updated_at: 2026-09-22T06:46:03Z
---

Review streaming, cancellation, metadata and exports; reconcile current main compatibility, fix confirmed issues with regression tests, remove committed fixture credentials, and validate tests/race/build/lint.

## Review findings and fixes

- P1: stream consumer failures could deadlock against a blocked Next; concurrent Rows.Close/Scan also raced. Streaming now owns a cancelable context and joins the sole row-reader/closer.
- P1: automatic query replay accepted executable MySQL/MariaDB comments, quoted variable assignments, qualified user functions and dialect-ambiguous expressions. Added fail-closed checks and regression tests without changing normal SELECT execution routing.
- P1: PostgreSQL pending edits ignored the captured database. Route transactions to the selected catalog and reject mixed-database batches before dispatch; verified with same-named tables in two live catalogs.
- P2: streamed/visible query CSV exports erased literal NULL&, EMPTY&, DEFAULT& strings. Separate raw query batches from Records display-marker cleanup and snapshot visible rows on the UI loop.
- P2: automatic count setup read UI pagination from workers. Move setup to the UI loop; synchronize test assertions/teardown and enable race detection in CI.
- P2: bulk metadata could restore stale results after refresh/DDL. Add cache generation checks and context ownership/cancellation for schema work.
- Integration: merge current main, preserve reverse Foreign Key Jump as a lazy cached operation, and adapt ClickHouse to context, pool, page-lookahead/count and typed streaming contracts.
- Security: replace tracked fixture credentials with random generated git-ignored 0600 files; bind ports to localhost. Historical GitGuardian incidents still require external review/dismissal as development-only credentials, or owner-authorized history cleanup.

## Validation

Passed go test ./..., go test -race ./... -count=3, go build ./..., golangci-lint v2.12.2 (0 issues), git diff --check, Compose validation, full RTT benchmark matrix, live four-provider fixture tests under the race detector, and live ClickHouse integration. Latest schema cancellation regression additionally passed three race-enabled repetitions.

## Publication

Local fixes prepared on fix/pr346-production in /private/tmp/lazysql-pr346-review. Remote CI/security gates must be rechecked after publication.

## Summary of Changes

Published c75d799 to feat/network-performance with operator approval using a normal non-force push. GitHub now reports MERGEABLE. CI run 35695242311 passed lint, tests, race detector, performance contracts, and build.

Follow-up: simplified required Compose interpolation to ${LAZYSQL_FIXTURE_PASSWORD:?}; GitGuardian incorrectly identified the previous interpolation error message as a Generic Password (incident 37513126). No generated local passwords were committed. Historical development-fixture incidents 37224298, 37224299, 37224300 and interpolation false positive 37513126 require owner review in GitGuardian. No history rewrite or scanner suppression was performed. Code-review remediation and validation are complete; security-check clearance remains an external merge gate.

## Concurrent main update

While the first validated update was publishing, main advanced to fd5ea57 by merging PR 330. Reconciled the database-selection API with context-aware reads/writes and streaming. SQL editor runs capture the selected database, and replay exports retain that original database even if selection changes. Added live cross-database streaming and reserved-session cancellation coverage.

Restarting the live fixture stack also reproduced SQL Server accepting SELECT 1 before user-database recovery completed. The old seed script treated lookup errors as missing fixtures and attempted to reseed. It now retries readiness and refuses to seed after errors, preserving manual data.

## Final merge validation

Revalidated the fd5ea57 main integration with the full unit suite, three repeated full race-detector runs, build, zero-issue lint, RTT benchmark matrix, live four-provider fixture checks, live cross-database read/write/stream isolation on MySQL/MSSQL/PostgreSQL, and race-enabled ClickHouse typed/capped streaming integration. A full down/up restart retained a SQL Server fixture sentinel, confirming seed recovery does not destroy data. Publication includes the new main merge; remote CI must run on that merge. Historical GitGuardian incident review remains external.
