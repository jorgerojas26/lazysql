---
# lazysql-7xye
title: Add transparent opt-out aggregate product telemetry
status: completed
type: feature
priority: normal
created_at: 2026-10-01T01:44:50Z
updated_at: 2026-10-01T02:05:31Z
---

- [x] Inspect existing consent, client, collector, actions and distribution setup
- [x] Implement disclosure and bounded product counters
- [x] Update aggregate collector validation and schema
- [x] Add tests and update privacy documentation

## Summary of Changes

Updated feat/anonymous-telemetry in /Users/jorgerojas/projects/lazysql-telemetry. Added first-draw-gated opt-out disclosure, bounded engine/failure/startup/read-only/distribution metrics, ten fixed action counters with five-minute lossy in-memory deltas, typed failure classification and GoReleaser/Homebrew channel detection. Added aggregate schema migration preserving existing counts, strict Worker validation, and privacy/rollout documentation. Preserved endpoint-bound global preferences, fail-closed behavior and all disable gates.

Validation: go test ./..., focused race tests, go vet ./..., go build ./..., golangci-lint v2.12.2 (0 issues), collector tests (13 passing), and git diff --check. No remote migration, deployment or release performed.
