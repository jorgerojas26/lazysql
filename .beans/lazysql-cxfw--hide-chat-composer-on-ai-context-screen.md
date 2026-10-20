---
# lazysql-cxfw
title: Hide chat composer on AI context screen
status: completed
type: bug
priority: normal
created_at: 2026-10-20T21:15:19Z
updated_at: 2026-10-20T21:16:14Z
---

Hide the chat input in context mode, remove it from focus traversal, and restore the draft composer when returning to chat. Verify layout and navigation.

## Summary of Changes

Hid the message input on both context tabs and reclaimed its layout space. Removed the hidden input from focus traversal and redirected background/reopen focus to the selected context view. Returning to chat restores the original draft and input focus. Added simulation-screen and focus regression tests.

Validation: go test ./... and targeted race-enabled context/streaming tests pass.
