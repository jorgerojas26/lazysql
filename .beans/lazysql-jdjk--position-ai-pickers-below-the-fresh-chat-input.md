---
# lazysql-jdjk
title: Position AI pickers below the fresh chat input
status: completed
type: bug
priority: normal
created_at: 2026-10-20T21:53:31Z
updated_at: 2026-10-20T21:54:06Z
---

Anchor model/thinking pickers below the input when space allows, retaining an above-input fallback near the screen bottom and bounded layout on small terminals. Add position regressions.

## Summary of Changes

Model and thinking pickers now prefer an anchor directly below the message input. When the popup would exceed the terminal bottom, it falls back above the input and remains clamped to screen bounds. Added fresh-chat below-input and constrained expanded-chat fallback position regressions.

Validation: go test ./..., targeted race-enabled picker rendering tests, and git diff --check pass.
