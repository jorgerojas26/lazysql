---
# lazysql-yjk6
title: Prevent expanded secrets in saved local connections
status: completed
type: bug
priority: normal
created_at: 2026-09-29T15:27:46Z
updated_at: 2026-09-29T15:43:52Z
---

PR #358 review follow-up: SaveConnections must preserve literal ${env:...} templates from local config instead of persisting expanded credentials when another connection changes.

- [x] Preserve raw local connection fields that the user did not change.
- [x] Add regression tests for edit/delete/add and secret-free disk output.
- [x] Test local database = [] after a fresh LoadConfig still hides global connections.
- [x] Run relevant tests and commit the fix to the PR branch.

## Summary of Changes

Preserved raw connection values for unchanged fields, added secret-leak and reload regression tests, and committed as a90ecf4. `go test ./...`, `go test -race ./...`, `go vet ./...`, and golangci-lint v2.12.2 for `./app` pass.
