---
# lazysql-2ipx
title: Resolve PR 360 review notes and merge main safely
status: completed
type: bug
priority: high
created_at: 2026-10-04T21:20:47Z
updated_at: 2026-10-04T21:45:25Z
---

Address current review findings and publish to the contributor PR branch.

- [x] Add regressions for Unicode controls and hidden-column sidebar raw copies
- [x] Integrate main preserving column visibility and binary raw values
- [x] Fix Unicode detection and duplicated sentinel rendering
- [x] Pass full validation and MySQL integration
- [x] Commit and push to TonyWu2333/lazysql fix/binary-hex-display

## Validation blocker

Full lint exposed seven existing main issues in updater and column visibility code. Resolve minimally before publication.

## Validation

Unicode C1 regressions failed before the fix and pass afterward. Full Go tests/race/vet/build, isolated golangci-lint v2.12.2 (0 issues), performance contract, binary/column-visibility regressions repeated five times with race, 13 telemetry collector tests, and disposable MySQL 8.4 integration with race all pass. Logs: /tmp/lazysql-pr360-fix-validation.

## Summary of Changes

Merged main 1950cd1 while preserving column visibility and binary sidebar raw-copy initialization. Fixed Unicode control detection and redundant sentinel rendering. Added query C1 controls, sentinel styles/references and hidden-column sidebar copy/edit regressions. Corrected inherited lint issues in the updater and column visibility tests. Full local validation and disposable MySQL integration pass. Published merge/fix commit 8781efa to TonyWu2333/lazysql fix/binary-hex-display without force push.
