---
# lazysql-abji
title: Unblock CI lint for row-copy PR
status: in-progress
type: bug
created_at: 2026-09-25T04:50:15Z
updated_at: 2026-09-25T04:50:15Z
---

PR #357 lint job never started: GitHub reports the account is locked due to a billing issue. Local v2.12.2 lint found two US-spelling warnings, now fixed.

- [x] Reproduce lint locally with CI version and fix warnings
- [ ] Push verified lint fix to the PR
- [ ] Restore GitHub Actions billing access and confirm green CI (requires account owner)
