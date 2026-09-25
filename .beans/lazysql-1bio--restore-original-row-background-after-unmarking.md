---
# lazysql-1bio
title: Restore original row background after unmarking
status: completed
type: bug
priority: normal
created_at: 2026-09-25T04:40:26Z
updated_at: 2026-09-25T04:43:19Z
---

Visual v preview uses SetBackgroundColor, which forces tview cells opaque. Cancel restored only BackgroundColor, not Style or Transparent, so cells retained the preview background. Space unmark is unaffected. Restore the full cell presentation.

- [x] Reproduce background artifact
- [x] Restore original cell style and transparency after visual selection
- [x] Add regression tests and run suite

## Summary of Changes

Visual range now snapshots and restores complete cell style, legacy background and transparency rather than a color alone. Regression tests cover leaving a range, Esc cancel, and pre-existing Space marks. go test ./... passes.
