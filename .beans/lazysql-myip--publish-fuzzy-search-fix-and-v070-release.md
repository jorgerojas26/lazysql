---
# lazysql-myip
title: Publish fuzzy search fix and v0.7.0 release
status: completed
type: task
priority: normal
created_at: 2026-10-12T15:30:03Z
updated_at: 2026-10-12T15:38:45Z
---

Publish the fix for https://github.com/jorgerojas26/lazysql/issues/364 and the next minor release requested by the user.

- [x] Open a dedicated fix PR with regression coverage
- [x] Validate and merge the PR
- [x] Publish v0.7.0 binaries, checksums, and Homebrew formula
- [x] Verify publication

## Summary of Changes

- Branch: fix/364-restore-fuzzy-table-search.
- PR: https://github.com/jorgerojas26/lazysql/pull/365 (merged; closes #364).
- Release: https://github.com/jorgerojas26/lazysql/releases/tag/v0.7.0 (latest stable).
- Tagged release commit: 91b8008307fe8e761a6cc1ad3a2db8aefc8ab5e5.
- Published eight GoReleaser platform archives and lazysql_0.7.0_checksums.txt. All downloaded archive checksums passed.
- Verified an unauthenticated public macOS arm64 download and its checksum; binary reports 0.7.0.
- Homebrew formula updated in jorgerojas26/homebrew-lazysql, commit d9da01057cadb465a774d20a6ae5157d91d9a583; Ruby syntax passes.
- Full Go tests and race tests, vet, build, CI-pinned golangci-lint v2.12.2 (0 issues), network performance contract, and all 13 collector tests passed locally.
- GitHub Actions remains blocked by the account billing lock before any steps execute. Release artifacts were built locally in a clean detached worktree and published after checksum verification.
- Preserved the configured public telemetry endpoint and official release distribution build flags.
- Local-only telemetry history commit was left on local main and excluded from the PR and release.
