---
# lazysql-79l4
title: Toggle AI model and thinking pickers with their shortcuts
status: completed
type: feature
priority: normal
created_at: 2026-10-20T21:51:11Z
updated_at: 2026-10-20T21:52:03Z
---

Ctrl+P toggles the model picker and Ctrl+T toggles the thinking picker. Repeated shortcut cancels without changing the selected setting or draft. Cover modal-global routing and picker keyboard handling.

## Summary of Changes

Ctrl+P and Ctrl+T now toggle their respective pickers closed without applying a selection and restore composer focus. Pressing the other selector shortcut switches pickers. Updated application-level Ctrl+T routing so the global theme shortcut does not swallow the close toggle, and removed the conflicting Ctrl+P search-navigation alias. Added toggle, cancellation, switching, and global-routing regressions.

Validation: go test ./..., targeted race-enabled picker/shortcut tests, and git diff --check pass.
