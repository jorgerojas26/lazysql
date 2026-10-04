---
# lazysql-2ipx
title: Resolve PR 360 review notes and merge main safely
status: in-progress
type: bug
priority: high
created_at: 2026-10-04T21:20:47Z
updated_at: 2026-10-04T21:44:32Z
---

Address current review findings and publish to the contributor PR branch.

- [x] Add regressions for Unicode controls and hidden-column sidebar raw copies
- [x] Integrate main preserving column visibility and binary raw values
- [x] Fix Unicode detection and duplicated sentinel rendering
- [x] Pass full validation and MySQL integration
- [ ] Commit and push to TonyWu2333/lazysql fix/binary-hex-display

## Validation blocker

Full lint exposed seven existing main issues in updater and column visibility code. Resolve minimally before publication.

## Validation

Unicode C1 regressions failed before the fix and pass afterward. Full Go tests/race/vet/build, isolated golangci-lint v2.12.2 (0 issues), performance contract, binary/column-visibility regressions repeated five times with race, 13 telemetry collector tests, and disposable MySQL 8.4 integration with race all pass. Logs: /tmp/lazysql-pr360-fix-validation.
