---
# lazysql-9or2
title: Clear baseline lint findings before v0.7.0
status: completed
type: task
priority: normal
created_at: 2026-10-12T15:30:33Z
updated_at: 2026-10-12T15:31:27Z
---

Clear pre-existing lint findings before publishing v0.7.0.

- [x] Explicitly discard read-only close errors
- [x] Remove builtin shadowing, spelling findings, and redundant embedded selector
- [x] Pass full lint and tests

## Summary of Changes

Applied behavior-preserving cleanup to updater and column visibility code. Full golangci-lint v2.12.2 reports 0 issues; full tests, race tests, vet, build, formatting, and diff checks pass.
