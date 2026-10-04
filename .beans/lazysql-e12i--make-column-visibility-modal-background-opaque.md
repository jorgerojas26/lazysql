---
# lazysql-e12i
title: Make column visibility modal background opaque
status: completed
type: bug
priority: normal
created_at: 2026-10-04T14:02:17Z
updated_at: 2026-10-04T14:07:13Z
---

The column visibility modal lets underlying content show through its padding and edges. Match the solid surface used by components/help_modal.go.

- [x] Reproduce background bleed with a simulated terminal test
- [ ] Make the complete modal panel opaque without painting the surrounding backdrop
- [x] Verify rendering regression and existing tests

## Additional requirement

- [x] Simplify the cluttered footer: count in the title, one short hint line, compact action buttons

## Summary of Changes

- Reproduced underlying content showing through panel padding with a filled simulated terminal screen. The Flex panel used a transparent Box by default, unlike the help modal's opaque Table surface.
- Replaced only the panel's Box with a normal opaque Box and matched the help modal's border color. Padding and button spacers now have a solid background while the surrounding backdrop stays visible.
- Reduced the bottom from four rows to two: one short Space toggle / find hint and compact All (A), Apply Enter, and Cancel Esc buttons. The draft visible-column count moved into the title, alongside the table name. Removed the extra blank list row.
- Added regression tests for full-panel opacity and compact footer/title updates. Verified existing keyboard/mouse behavior, go test ./..., go vet ./..., and five focused race-detector runs.
