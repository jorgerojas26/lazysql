---
# lazysql-8lih
title: Hide columns per connection and table
status: completed
type: feature
priority: normal
created_at: 2026-10-04T12:56:55Z
updated_at: 2026-10-04T13:22:51Z
---

Add compact, discoverable client-side column visibility with mnemonic keyboard shortcuts and mouse support. Persist preferences per connection/database/table, without changing queries or row operations.

- [x] Inspect UI and persistence; design compact interaction
- [x] Implement modal, display projection, and persistence
- [x] Add regression coverage and verify tests

## Summary of Changes

- Added a mnemonic V (visibility) shortcut and clickable Columns [V] label on the existing pagination border, with a hidden-column count and no extra permanent rows.
- Added a compact, responsive, searchable checkbox modal with Space/click toggles, A/Show all, Enter/Apply, Esc/Cancel, keyboard focus cycling, and mouse-safe modal behavior.
- Persisted hidden column names per connection, database, and schema-qualified table in the active configuration, preserving credential templates and local/global isolation.
- Projected the existing cell objects for display only: primary keys, pending edits, marks, foreign key markers, sidebar data, complete row copies, and exports remain intact. Navigation and inline editing skip hidden columns; same-table tabs update together. New columns remain visible and at least one column is kept.
- Documented the feature and added persistence, keyboard/mouse, compact layout, editing/inserts, data-integrity, and terminal simulation regression coverage.
- Verified go test ./..., go vet ./..., and go test -race ./app ./components -run HiddenColumns\|ColumnVisibility -count=5.
