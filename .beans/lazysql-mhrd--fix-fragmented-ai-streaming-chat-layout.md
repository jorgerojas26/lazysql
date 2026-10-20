---
# lazysql-mhrd
title: Fix fragmented AI streaming chat layout
status: completed
type: bug
priority: normal
created_at: 2026-10-20T20:58:19Z
updated_at: 2026-10-20T21:00:11Z
---

Streamed responses appear in a separate clipped pane below a large blank gap. Make replies flow continuously in the chat and cover layout and scrolling with regression tests.

## Summary of Changes

Replaced the split transcript/live-reply panes with one scrollable conversation. Streaming SQL now follows the prompt without blank gaps and uses the full viewport. Preserved cached historical SQL highlighting, non-clickable partial SQL, follow-at-bottom behavior, and manual scroll position during updates. Added simulation-screen regressions for responsive layout, growing SQL, scrollback, and stable completion.

Validation: go test ./... and targeted race-enabled chat/stream/query tests pass.
