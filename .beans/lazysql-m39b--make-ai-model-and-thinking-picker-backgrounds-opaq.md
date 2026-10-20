---
# lazysql-m39b
title: Make AI model and thinking picker backgrounds opaque
status: completed
type: bug
priority: normal
created_at: 2026-10-20T21:47:14Z
updated_at: 2026-10-20T21:49:19Z
---

Match solid modal rendering used by existing dialogs. Prevent underlying chat from bleeding through picker padding/empty areas while preserving content outside popup bounds. Add model/thinking rendering regressions.

## Additional requirement

Remove the search field from the thinking picker; focus its list directly and keep model search unchanged.

## Summary of Changes

Replaced the picker Flex background box with a normal clearing Box, matching the solid modal pattern rather than Flex default transparency. Model and thinking pickers now fully paint their borders, padding, and empty areas while preserving all content outside popup bounds. Removed the thinking search row, focused the list directly, kept Tab on the list, and retained arrow/Enter selection. Model search remains unchanged. Empty status rows no longer consume space.

Validation: added cell-by-cell opacity/outside-bounds rendering regressions at multiple widths and thinking-list keyboard tests. go test ./..., targeted race checks, and git diff --check pass.
