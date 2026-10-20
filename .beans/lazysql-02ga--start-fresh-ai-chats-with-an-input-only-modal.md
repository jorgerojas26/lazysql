---
# lazysql-02ga
title: Start fresh AI chats with an input-only modal
status: completed
type: feature
priority: normal
created_at: 2026-10-20T21:18:03Z
updated_at: 2026-10-20T21:20:18Z
---

First open and New chat should show only a compact message input. Sending expands to the existing chat layout; drafts and context view remain intact. Add responsive and transition tests.

## Summary of Changes

Fresh chats and New chat now render only a centered message input, with no outer modal frame, transcript, status, context meter, or action buttons. Sending expands to the previous full chat layout; context remains available via Ctrl+K. Drafts survive hide/reopen and context navigation, hidden controls are excluded from focus traversal, and blank submissions leave the minimal view unchanged. Added responsive layout, reset/expansion, and real streaming lifecycle regression coverage.

Validation: go test ./... and targeted race-enabled fresh-chat, context, overlay, and streaming tests pass. Prior context-screen changes committed first as d18acf0; minimal-chat changes remain uncommitted.
