---
# lazysql-2vrh
title: Reuse TabbedPane in AI context screen
status: completed
type: task
priority: normal
created_at: 2026-10-20T21:12:05Z
updated_at: 2026-10-20T21:14:07Z
---

Replace bespoke Usage/Conversation buttons and Pages with the existing TabbedPane and shared header styling. Retain keyboard and mouse navigation and context payload inspection.

## Summary of Changes

Replaced custom context tab buttons and Pages with the existing TabbedPane and HeaderContainer. Usage/Conversation now share the standard active-tab styling and responsive header layout. Preserved clicks, keyboard focus/Enter, numeric shortcuts, and added standard configured previous/next-tab navigation without intercepting composer typing. Added header styling and navigation regression coverage.

Validation: go test ./... and targeted race-enabled context/streaming tests pass.
