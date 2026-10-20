---
# lazysql-5gtn
title: Add Ctrl+U/D chat scrolling and move Use query to Ctrl+Y
status: completed
type: feature
priority: normal
created_at: 2026-10-20T21:22:16Z
updated_at: 2026-10-20T21:23:51Z
---

Ctrl+U and Ctrl+D scroll the AI chat by half a page while retaining input focus and draft. Move Use query to user-selected Ctrl+Y. Update button labels and regression tests.

## Summary of Changes

Bound Ctrl+U/Ctrl+D to half-page up/down scrolling in the chat or selected context tab while preserving the composer draft and current focus. Empty compact chats ignore scroll commands. Scrollback stays stable during streaming, and reaching the bottom resumes following. Moved Use query to the user-selected Ctrl+Y shortcut and updated labels and acceptance tests. Added scrolling, boundary, draft/focus, context, and streaming regressions.

Validation: go test ./... and targeted race-enabled scrolling/acceptance/streaming tests pass.
