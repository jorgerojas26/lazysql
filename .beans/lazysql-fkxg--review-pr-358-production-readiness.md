---
# lazysql-fkxg
title: Review PR 358 production readiness
status: completed
type: task
priority: normal
created_at: 2026-09-30T23:23:40Z
updated_at: 2026-09-30T23:31:01Z
---

Review exact GitHub PR head 334a23b without changing source.

- [x] Inspect scope and requirements
- [x] Review standards and specification directly (user-approved fallback; reviewers failed before launch)
- [x] Validate tests and report production verdict

## Review evidence

Exact published PR head remains 334a23b. No standards violations found. SaveConnections persists expanded local env credentials when adding/editing/deleting another connection; reproduced all three cases with synthetic credentials. This is residual pre-existing security behavior, not a new regression. Existing suite, vet, full race suite, build and golangci-lint v2.12.2 with fresh cache pass. GitHub CI did not start because of account billing lock. Parallel workflow c2fa85cf-2995-435f-a2c5-995ac3f824e7 failed before both child launches with workflowRunParamsFingerprint is not a function; user authorized direct review.

## Summary of Changes

Review only; no source edits, commits, pushes or PR comments. Standards: 0 findings. Spec/security: 1 P1 residual credentials leak in app/config.go:258-266; confirmed it also exists on base main. Recommend not approving the published head as a complete production-ready security fix until unchanged local env templates are preserved. Local checkout commit 14a1b4b already preserves templates in the three reproductions, but GitHub PR still points to 334a23b. Full tests, race tests, vet, build and pinned lint pass on exact PR head; empty local connections remain empty after reload.

Reproduction script: /tmp/lazysql-pr358-repro.go; captured diff: /tmp/lazysql-pr358.diff.
