---
# lazysql-wsdt
title: Match tree search substrings and navigate in tree order
status: in-progress
type: bug
priority: normal
created_at: 2026-10-04T12:53:04Z
updated_at: 2026-10-04T20:01:56Z
---

Follow-up to lazysql-arxv: searching user incorrectly includes BusinessProgram, and n/p jumps between distant results instead of following tree order.

- [x] Reproduce false-positive matching and non-contiguous navigation
- [x] Require contiguous case-insensitive matches and nearest-result navigation
- [x] Validate regression tests and the full suite

## Diagnosis

Deterministic UI-path tests reproduce user matching the sole BusinessProgram node and n selecting user_profiles instead of the next result in tree order. Separate simulated-screen tests show that manual j movement followed by p, or jj followed by n/p, navigates from stale currentFocusFoundNode instead of the actual cursor. Search still uses fuzzy subsequence matching and sorts searchFoundNodes by match relevance.

## Summary of Changes

- Replace fuzzy subsequence matching in tree searches with case-insensitive contiguous substring matching for both object names and ancestor qualifiers. Searching user no longer includes BusinessProgram or us_ers.
- Keep results sorted by relevance and initially select the best match; n/p uses tree position independently of relevance.
- Navigate n/p relative to the actual current cursor, including after manual j/k movement and from non-matching nodes. Wrap only after the last/before the first spatial match, expand ancestors, and keep counters aligned with the selected ranked result.
- Reject empty patterns such as separator-only queries and trim surrounding query whitespace.
- Preserve scoped qualified searches, their existing exact-match narrowing, and the prior concurrency fix.
- Add six regression tests for the reported false positive, mixed-relevance navigation, manual cursor movement, literal/case-insensitive names and qualifiers, and empty patterns. Update prior ordering and partial-search tests to the new contract.

Validation: failing reproductions were run before the fix; search regressions passed 50 times under the race detector with actual key handlers and simulated rendering. go test ./..., go test -race ./..., go vet ./..., go build ./..., and git diff --check passed.

## Ranking clarification

The user clarified that relevance ordering and the initial best match must remain; only n/p navigation should follow tree position from the current cursor. Keep literal matching.

- [x] Restore relevance ranking and best-match initial selection
- [x] Validate spatial navigation independently of ranking and rerun the suite

## Ranking Correction Summary

Restored the original exact/prefix/substring relevance scoring and ancestor weighting with stable tie ordering. Initial focus selects the best match. Navigation chooses the nearest node in the requested tree direction with circular wrapping, without relying on or mutating the ranked result list. Literal matching remains unchanged. Regression tests verify relevance order, initial best hit, spatial n/p cycles, ranking preservation during navigation, ancestor relevance, and stable ties. Targeted tests passed 25 times with -race; full normal/race suites, vet, build, and diff checks passed.

## PR Publication

User requested publication of the validated tree search fix.

- [x] Revalidate the final branch and diff
- [ ] Commit the code and beans, push the branch, and open the PR against main

Final publication checks on fix/tree-search-results-and-navigation: uncached normal and race suites, go vet, go build, formatting/diff checks, and golangci-lint v2.12.2 all passed. The matching CI linter was run with an isolated cache and reported 0 issues.
