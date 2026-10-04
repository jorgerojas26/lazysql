---
# lazysql-bes7
title: Fix telemetry test isolation after metadata merge
status: completed
type: bug
priority: normal
created_at: 2026-10-01T02:23:38Z
updated_at: 2026-10-01T02:25:37Z
blocking:
    - lazysql-oi28
---

- [x] Reproduce race between telemetry test application replacement and asynchronous metadata updates
- [x] Isolate telemetry test screen/callbacks without replacing the global application
- [x] Verify targeted and full race tests

## Summary of Changes

Kept the global tview application instance stable in telemetry UI tests, drew disclosures directly onto a test-owned simulation screen, and restored focus/input/draw callbacks during cleanup. This prevents replacement of app.App.Application racing with metadata callbacks introduced by main. Reproduced with go test -race ./components -run "TestRecordsMetadataStartsKindsConcurrently|TestTelemetryDisclosureGateAndPreferences" -count=5; the same loop now passes for 10 repetitions. Full components and telemetry race suites, vet, build and lint pass.
