---
# lazysql-myip
title: Publish fuzzy search fix and v0.7.0 release
status: in-progress
type: task
priority: normal
created_at: 2026-10-12T15:30:03Z
updated_at: 2026-10-12T15:32:19Z
---

Publish the fix for https://github.com/jorgerojas26/lazysql/issues/364 and the next minor release requested by the user.

- [x] Open a dedicated fix PR with regression coverage
- [ ] Validate and merge the PR
- [ ] Publish v0.7.0 binaries, checksums, and Homebrew formula
- [ ] Verify publication

Keep the local-only telemetry commit out of the PR and release.

## Pull request

https://github.com/jorgerojas26/lazysql/pull/365

Full Go tests and race tests, vet, build, full golangci-lint v2.12.2 (0 issues), and 13 collector tests passed. Local-only telemetry commit remains on local main, outside this branch.
