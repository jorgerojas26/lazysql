---
# lazysql-abji
title: Unblock CI lint for row-copy PR
status: in-progress
type: bug
priority: normal
created_at: 2026-09-25T04:50:15Z
updated_at: 2026-09-25T04:50:45Z
---

PR #357 lint job never started: GitHub reports the account is locked due to a billing issue. Local v2.12.2 lint found two US-spelling warnings, now fixed.

- [x] Reproduce lint locally with CI version and fix warnings
- [x] Push verified lint fix to the PR
- [ ] Restore GitHub Actions billing access and confirm green CI (requires account owner)

## Progress

Pushed commit 30f8dc6 to PR #357. Local golangci-lint v2.12.2 reports 0 issues; go test ./... passes. GitHub Actions remains blocked by the account billing lock; no runner steps have executed.
