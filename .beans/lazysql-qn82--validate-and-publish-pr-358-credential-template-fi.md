---
# lazysql-qn82
title: Validate and publish PR 358 credential template fix
status: completed
type: task
priority: normal
created_at: 2026-09-30T23:33:41Z
updated_at: 2026-09-30T23:37:23Z
---

User requested implementation, commit and push after PR review. Existing local commit 14a1b4b implements template preservation; publish to the actual contributor PR branch after validation.

- [x] Verify PR branch and existing fix
- [x] Add independent credential leak regression cases
- [x] Run tests, race detector, vet, build and pinned lint
- [x] Commit and push to PR branch

## Implementation evidence

Retained existing implementation commit 14a1b4b. Added separate local/global add/edit/delete/rename cases, requested-change reload assertions, untouched-file checks, MSSQL raw Username/Password template and generated-URL leak coverage. README documents unchanged-field placeholder preservation. New tests fail against published PR config.go through a Go overlay and pass against the fixed implementation. Actual PR target is git@github.com:anandghegde/lazysql.git, fix/local-config-save, maintainer edits enabled.

## Validation

Passed go test ./..., go test -race ./..., go vet ./..., go build ./..., git diff --check and golangci-lint v2.12.2 (fresh cache, 0 issues).

## Summary of Changes

Published existing credential-template fix 14a1b4b and regression/documentation commit 19ea926 to anandghegde/lazysql:fix/local-config-save with a fast-forward push. Added eight separate local/global add/edit/delete/rename cases plus MSSQL credential-field coverage. Tests assert untouched config isolation, no expanded secrets, persisted requested edits and reload behavior. README explains placeholder preservation. Full suite, full race suite, vet, build and pinned lint pass. CI billing lock is external to this correction.
