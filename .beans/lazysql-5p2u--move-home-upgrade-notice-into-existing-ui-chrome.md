---
# lazysql-5p2u
title: Move home upgrade notice into existing UI chrome
status: completed
type: task
priority: normal
created_at: 2026-10-05T08:05:48Z
updated_at: 2026-10-05T08:15:28Z
---

Keep the upgrade notice on the connection selection screen, but remove its dedicated row in the connected home view. Display it in existing UI chrome without using extra rows.

- [x] Inspect notice and home layout
- [x] Relocate the home notice without extra screen space
- [x] Add regression tests and run checks

The user prefers the home notice on the existing bottom-right border, not the top border.

Refined behavior: keep only the current running version on the bottom-right home border. Newly available upgrades appear in a transient, non-focus-stealing toast. Preserve the connection-selection footer.

- [x] Add transient upgrade availability toast

## Summary of Changes

Removed the reserved update footer row from connected home views. The existing bottom-right border now shows only the running LazySQL version. Available upgrades produce an eight-second toast with the update shortcut without taking focus or changing layout; toasts do not draw over dialogs. Connection selection retains its full update footer. Added simulated-screen regressions for picker/home transitions, direct home startup, dialog overlays, current-version placement, and toast display/expiration.

Validation: go test ./..., go vet ./..., focused race tests, and git diff --check passed.
