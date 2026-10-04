---
# lazysql-arxv
title: Fix inflated tree search matches and result navigation
status: completed
type: bug
priority: normal
created_at: 2026-10-04T12:36:59Z
updated_at: 2026-10-04T12:49:12Z
---

Reproduce and fix tree searches that report too many matches and navigate through unrelated nodes.

- [x] Reproduce incorrect counts and navigation with deterministic tests
- [x] Fix the root cause and add regression coverage
- [x] Validate counts, result navigation, and the full test suite

## Diagnosis

Rapid filter edits launch concurrent searches from the UI callback. Results from older queries are appended into the same searchFoundNodes slice, duplicating nodes and leaking stale matches. Reproduction: a final dbA.users query yielded 16,764 entries for one matching table; two edits and two tables yielded 3 entries. The race detector confirms overlapping writes in Tree.search and tview.TreeNode.Walk. Result navigation also fails to expand ancestors of subsequent matches.

## Summary of Changes

- Execute filter searches synchronously on the UI loop instead of spawning a goroutine for every edit, preventing duplicated results and stale query completion.
- Expand ancestors when navigating next/previous results so drawing preserves the selected match.
- Reset search focus for empty/no-match searches and show [0/0] instead of stale counts.
- Keep tree order among equal-ranked results using stable sorting.
- Add six regression tests exercising real filter callbacks, Enter/Escape handlers, n/N navigation with a simulated screen, 1,000-table rapid typing, empty/no-match state, and ranking ties.

Validation: the original stress reproduction now returns exactly one unique result; targeted regression tests passed 25 times with the race detector; go test ./..., go test -race ./..., go vet ./..., go build ./..., and git diff --check all passed.
