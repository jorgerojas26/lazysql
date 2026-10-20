---
# lazysql-yo0l
title: Remove AI byte context cap and simplify context indicator
status: completed
type: task
priority: normal
created_at: 2026-10-20T20:55:15Z
updated_at: 2026-10-20T20:56:31Z
---

Remove the 96 KB context guard and byte-driven history trimming. Move context below chat input and display percentage/model window in Pi style. Add regression tests.

## Summary of Changes

Removed the fixed context byte cap, byte-based history pruning, and derived chat input length cap. Moved context usage directly below the message input and formatted known model windows as percentage/window (for example 5%/500k), with estimated token fallback for unknown windows. Removed KB budget text and byte-based warning thresholds. Added regressions for large requests, history preservation, layout, and formatting.

Validation: go test ./... passes.
