---
# lazysql-h36r
title: '[05.1] Preserve pending metadata enrichment across surface Refresh'
status: completed
type: bug
priority: high
tags:
    - ready-for-agent
    - order-05-01
    - review-finding
    - review-cycle-0001
created_at: 2026-09-12T08:38:43Z
updated_at: 2026-09-12T09:23:59Z
parent: lazysql-a9lf
---

## Review source

Review of `a22f2ed12ee9e98592c5801226d56a80bb25a792...HEAD`.
Pinned reviewed HEAD: `2ab40b88a5b20570e1613cc391f219a946015658`.
Diff command: `git diff a22f2ed12ee9e98592c5801226d56a80bb25a792...2ab40b88a5b20570e1613cc391f219a946015658`.

Owning Bean: `lazysql-fwt5 — [05] Make Refresh reload only the active surface`
Introducing commit: `2ab40b8`.

Axes:
- Standards: none; reported optional smells do not establish this defect.
- Spec: Refresh abandons unrelated in-flight metadata consumers, preventing progressive enrichment of the unchanged table.

## Observed behavior

Open Records with PK/FK and other structural metadata still pending. Switch to Columns and press R before those unrelated queries finish. RefreshMetadata starts a new shared load generation and cancels the old context. Existing metadata consumers reject their completion; only Columns receives a replacement consumer. Returning to Records does not apply the successful unrelated cached results: local metadata can remain Loading and PK capabilities or Foreign Key Jump underline/navigation remain unavailable until further navigation/reload.

Records Refresh similarly starts a new load generation but subscribes only to PK metadata, losing other pending metadata consumers. Both symptoms share the same correction: preserve delivery of still-valid unrelated metadata across same-table refresh.

Source evidence: components/table_metadata.go:313 and 396–408; components/results_table.go:1212–1227; components/row_count.go:20–22. This is a static control-flow finding; existing affected tests passed but do not cover the pending-unrelated-metadata refresh sequence.

This violates lazysql-fwt5's requirement that R in each metadata surface invalidates/reloads only that kind, and regresses lazysql-846w's requirements that PK-dependent capabilities become available independently and Foreign Key Jump gains underline/navigation when metadata arrives without refetching Records. Same-table refresh is not a stale-table transition.

## What to build

Keep valid pending structural metadata capable of enriching the current table across Records and metadata-surface Refresh. Refresh only the requested surface and its allowed state, without abandoning unrelated pending results or issuing unrelated speculative database queries. Preserve cancellation/stale-result protection when the actual table identity changes. Reusing or reattaching consumers must not duplicate underlying metadata queries.

## Acceptance criteria

- [x] Reproduce the pending-unrelated-metadata defect deterministically with controlled metadata completion, and correct it.
- [x] After each metadata-surface Refresh while unrelated metadata is pending, successful unrelated results reach the unchanged table and leave Loading state; PK capabilities and Foreign Key Jump become available without another Records fetch or navigation.
- [x] Records Refresh while non-PK structural metadata is pending preserves its eventual delivery without refetching unrelated metadata.
- [x] Preserve lazysql-fwt5's active-surface query isolation, filter/sort/pagination preservation, row-count refresh, valid metadata reuse, and explicit failed-surface retry criteria.
- [x] Add or update regression coverage that fails on the reviewed implementation and passes after correction; assert exact underlying database calls rather than wall-clock timing.
- [x] Preserve lazysql-846w's per-kind independence, in-flight deduplication, valid late cache population, and protection against old-table results mutating a newer visible table.

## Blocked by

None. No implementation prerequisite or review-ordering dependency is required.

## Summary of Changes

- Decoupled metadata result consumers from Records/surface load generations so pending same-table metadata survives Refresh.
- Added table-identity generations and serialized metadata application with identity changes to reject stale results safely.
- Added deterministic refresh regressions for pending metadata delivery, exact driver calls, Foreign Key Jump enrichment, and stale identity transitions.
