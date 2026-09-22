---
# lazysql-pgoh
title: Review and harden PR 346 for production
status: in-progress
type: bug
priority: high
created_at: 2026-09-22T06:03:36Z
updated_at: 2026-09-22T06:29:53Z
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
