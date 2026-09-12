---
# lazysql-7txh
title: '[11.1] Preserve pending metadata across context-aware Refresh'
status: todo
type: bug
priority: high
tags:
    - ready-for-agent
    - order-11-01
    - review-finding
    - review-cycle-0002
created_at: 2026-09-12T20:42:44Z
updated_at: 2026-09-12T20:42:44Z
parent: lazysql-a9lf
---

## Review source

Review of `v0.5.7...HEAD` (fixed point `a07385a07523428234e350727f9e769f464e302e`, pinned HEAD `8bdaefccbb191f33a5285f7f25bec1a41b1b1c03`).

Owning Bean: `lazysql-xcd4 — [11] Contract legacy driver I/O into one context-aware API`

Axes:
- Standards: none
- Spec: Same-table Records or metadata-surface Refresh cancels pending structural metadata after the context-aware Driver contraction, regressing `lazysql-h36r`.

## Observed behavior

Pending metadata cache loaders run with the originating Records load context. Starting a same-table Refresh cancels that context. A real context-aware PK/FK/columns/constraints/indexes driver call therefore returns cancellation and the shared cache records a failed result instead of eventually enriching the unchanged table. The current pending-refresh fakes ignore `context.Context`, so their green tests do not exercise the contracted Driver behavior.

This violates `lazysql-h36r`'s requirements that unrelated pending metadata survive Records/surface Refresh and enable PK capabilities and Foreign Key Jump, and `lazysql-xcd4`'s requirement that existing metadata cancellation behavior remain green after API contraction.

## What to build

Give structural metadata work a lifetime that survives same-table Records and surface Refresh while remaining cancellable when the table/connection identity actually becomes stale. Users must receive eventual metadata enrichment without duplicate catalog queries, and navigating away or closing the relevant connection must still stop obsolete database work.

## Acceptance criteria

- [ ] Reproduce both Records Refresh and metadata-surface Refresh with a context-observing pending metadata driver, and correct the cancellation regression.
- [ ] Successful unrelated metadata reaches the unchanged table after Refresh, leaves Loading state, and enables PK capabilities or Foreign Key Jump as applicable.
- [ ] Same-table Refresh does not duplicate the pending underlying metadata query and preserves cache in-flight deduplication.
- [ ] A real table/connection identity change still cancels obsolete metadata database work and stale results cannot mutate the new table.
- [ ] Preserve the context-aware-only Driver API and `lazysql-h36r`'s active-surface isolation and exact-call invariants.
- [ ] Add regression coverage that fails on the reviewed implementation and passes after correction.

## Blocked by

None. No implementation prerequisite is required.
