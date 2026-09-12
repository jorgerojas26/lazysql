---
# lazysql-9cq4
title: '[09.1] Apply MSSQL schema filters across schema loading'
status: completed
type: bug
priority: high
tags:
    - ready-for-agent
    - order-09-01
    - review-finding
    - review-cycle-0002
created_at: 2026-09-12T20:42:44Z
updated_at: 2026-09-12T21:24:34Z
parent: lazysql-a9lf
---

## Review source

Review of `v0.5.7...HEAD` (fixed point `a07385a07523428234e350727f9e769f464e302e`, pinned HEAD `8bdaefccbb191f33a5285f7f25bec1a41b1b1c03`).

Owning Bean: `lazysql-fz2q — [09] Make SQL autocomplete progressive and remove schema N+1 queries`

Axes:
- Standards: none
- Spec: MSSQL schema filters are absent from shared schema/autocomplete loading and consequently from the progressive tree.

## Observed behavior

MSSQL table discovery returns bare names grouped only by database, declares schemas unsupported, and bulk-column lookup filters only by bare table name. Configured MSSQL schema exclusions therefore cannot prevent autocomplete/tree/catalog work for hidden schemas, and same-named tables in different schemas can be merged or attributed incorrectly.

This violates parent spec §35, `lazysql-fz2q`'s hidden/excluded-schema criterion, and `lazysql-gw9j`'s requirement that schema filters apply consistently to tree and schema loading.

## What to build

Preserve MSSQL schema identity throughout discovery and shared schema loading so configured exclusions are applied before hidden-schema catalog work is scheduled. Autocomplete, bulk and lazy column loading, and the progressive tree must agree on qualified table identity, including databases containing duplicate table names across schemas.

## Acceptance criteria

- [x] Configured MSSQL schema exclusions apply consistently to tree, autocomplete, bulk column loading, and lazy/on-demand column loading.
- [x] Hidden schemas generate no avoidable catalog traffic from these surfaces.
- [x] MSSQL table identities retain enough schema information to distinguish duplicate table names in different schemas.
- [x] Bulk and lazy column results are associated with the correct schema-qualified table without cross-schema merging.
- [x] Preserve progressive table-first rendering, shared-cache reuse, and supported behavior for PostgreSQL, MySQL, and SQLite.
- [x] Add regression coverage for excluded schemas and duplicate MSSQL table names that fails on the reviewed implementation and passes after correction.

## Blocked by

None. No implementation prerequisite is required.

## Summary of Changes

- Made MSSQL table discovery schema-aware and retained qualified identities through tree, autocomplete, bulk, and lazy metadata loading.
- Added schema-qualified MSSQL bulk catalog filtering, duplicate-safe autocomplete mapping, and qualified table reference handling for table operations.
- Added regression coverage for excluded schemas, duplicate table names, qualified bulk results, and filtered progressive tree references.
- Verified with `go test ./...`.
