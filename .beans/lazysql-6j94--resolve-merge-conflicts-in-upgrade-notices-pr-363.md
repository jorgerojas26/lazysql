---
# lazysql-6j94
title: Resolve merge conflicts in upgrade notices PR 363
status: completed
type: task
priority: normal
created_at: 2026-10-04T20:25:43Z
updated_at: 2026-10-04T20:27:35Z
---

Merge origin/main into feat/upgrade-notices while preserving telemetry, tree search and column visibility changes.

- [x] Resolve README.md, go.mod and main.go conflicts.
- [x] Run tests and merge validation.
- [x] Commit the resolution and update PR #363.

## Summary of Changes

Merged origin/main into feat/upgrade-notices in 46a812e and pushed to PR #363. Resolved README.md by preserving all application settings and upgrade docs, go.mod by retaining x/mod and x/term, and main.go by keeping telemetry preparation/cleanup around the update-enabled UI. Preserved automatic merge results for tree search, column visibility and release configuration. Validation passed: go test ./..., go vet ./..., selected updater/UI/telemetry tests with -race, and git diff --cached --check.
