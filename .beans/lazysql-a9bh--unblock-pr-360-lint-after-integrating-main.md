---
# lazysql-a9bh
title: Unblock PR 360 lint after integrating main
status: completed
type: task
priority: high
created_at: 2026-10-04T21:42:20Z
updated_at: 2026-10-04T21:44:32Z
---

Resolve seven lint errors inherited from main during PR 360 integration.

- [x] Correct unchecked closes, spelling, builtin shadow and redundant selector
- [x] Pass golangci-lint v2.12.2 and affected tests

## Summary of Changes

Explicitly discard read-side close errors, rename test variables, and remove the redundant embedded Table selector. Full tests/race/vet/build and golangci-lint v2.12.2 (0 issues) pass on the integrated PR tree.
