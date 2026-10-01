---
# lazysql-m4ft
title: Copy selected result rows in SQL, Markdown, JSON, CSV
status: completed
type: feature
priority: normal
created_at: 2026-09-25T04:18:43Z
updated_at: 2026-09-25T04:39:52Z
---

Add range marking alongside Space toggling, a Y format chooser for copying selected or current rows in both records tables and SQL editor results, keep lowercase y unchanged, add tests and help.

- [x] Inspect selection and clipboard conventions
- [x] Implement range marking and multi-format copy
- [x] Test and document keybindings

## Summary of Changes

Added a Y format picker to ResultsTable, shared by records and SQL editor results. Supports SQL, Markdown, JSON, CSV with y behavior unchanged. v now previews a live visual range; Y copies it directly, Space commits its marks, Esc cancels. Added tests and README help; go test ./... passes.

## Live visual range refinement

- [x] Replace anchored v with live row range preview
- [x] Let Y copy preview, Space commit, Esc cancel
- [x] Update tests and help for both result surfaces

## Keybinding refinement

- [x] Use lowercase v for live row selection in code, tests and docs
