---
# lazysql-xxa0
title: Fix binary hex editing and raw value regressions in PR 360
status: completed
type: bug
priority: high
created_at: 2026-10-04T20:47:20Z
updated_at: 2026-10-04T21:05:30Z
---

Resolve review findings: original-hex reversion, NULL/DEFAULT raw cache, row shifts, duplicate/external-editor binary writes, individual/sidebar raw copies. Cover printable binary metadata and regression tests. Validate and publish to PR branch.

## Summary of Changes

Resolved all five PR 360 review findings. Raw binary values are keyed by stable TableCell identity and retained across text/special edits. Hex decoding is centralized when accepting edits; inline, sidebar and external editor share it. Duplication reads raw values and inserted rows render them as hex. Single-cell and sidebar copies use current raw values. Binary column metadata formats printable/whitespace bytes and empty hex round-trips. Added UI, cache-lifecycle, copy, rejection and MySQL integration regression tests. Also synchronized a pre-existing refresh-test read race reproduced on the original PR head.

Validation: go test ./..., go test -race ./..., go vet ./..., go build ./..., git diff --check, golangci-lint v2.12.2 (0 issues), 20 race repetitions of the refresh regression, and real MySQL 8.4 integration including binary UPDATE and duplicate INSERT, with race detection, all pass.
