---
# lazysql-v3y2
title: Simplify AI context screen with usage and conversation tabs
status: completed
type: feature
priority: normal
created_at: 2026-10-20T21:06:26Z
updated_at: 2026-10-20T21:10:42Z
---

Show concise token usage instead of verbose context diagnostics. Add a separate conversation tab exposing full outbound model context and messages without credentials. Preserve navigation and test payload fidelity.

## Summary of Changes

Replaced verbose context diagnostics with a compact estimated-context and last/session token usage summary. Added Usage and Conversation tabs with mouse, Tab navigation, and 1/2 shortcuts outside the composer. Conversation displays the complete latest outbound JSON request, including instructions, conversation history, schema/tool definitions, replay, and metadata results. Captures actual serialized bodies for all supported protocols, excludes authentication headers, ignores stale runs, and clears snapshots on New chat. Unknown usage remains explicitly unreported.

Validation: go test ./... and targeted race-enabled UI and provider request-fidelity tests pass. Prior AI chat improvements committed separately as 9a4fe7d; this context-screen change remains uncommitted.
