---
# lazysql-gw9j
title: '[10] Make the database tree progressive and keep schema freshness correct'
status: todo
type: task
priority: normal
tags:
    - dml
    - ready-for-agent
    - network-performance
    - schema-loader
    - ddl
    - order-10
    - ticket-key-progressive-tree-schema-freshness
    - tree
created_at: 2026-09-12T03:19:45Z
updated_at: 2026-09-12T03:19:46Z
parent: lazysql-a9lf
blocked_by:
    - lazysql-mdze
    - lazysql-lone
    - lazysql-fz2q
---

## What to build

Apply the same progressive-loading and non-speculative-networking principles to the database tree and schema freshness.

Render databases and tables as soon as they are available, enrich programming objects afterward, reuse the shared schema loader/cache, invalidate schema metadata after successful DDL, and remove speculative Records refreshes after arbitrary editor DML.

Known DML originating from Records should still refresh the exact table's Records/count state while preserving structural metadata.

## Acceptance criteria

- [ ] Database nodes can render without waiting for every database's complete object metadata.
- [ ] Table nodes can render without waiting for functions, procedures, and views.
- [ ] Functions/procedures/views enrich the tree after primary table information is available.
- [ ] Tree loading reuses the shared connection-scoped schema loader/cache.
- [ ] Existing schema filters apply consistently to tree and schema loading.
- [ ] Successful arbitrary DDL invalidates the connection's schema metadata cache.
- [ ] After DDL invalidation, the visible tree rebuilds progressively in background without delaying SQL completion feedback.
- [ ] Arbitrary SQL-editor DML does not automatically refresh whichever table happened to be selected/opened.
- [ ] DML originating from Records refreshes the known affected table's Records and row-count state.
- [ ] Known Records DML preserves valid structural metadata cache.
- [ ] Tests cover progressive tree order, cache reuse, DDL invalidation/rebuild, removal of speculative editor-DML refresh, and known-table DML refresh.
