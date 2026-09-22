---
# lazysql-532r
title: '[06] Eliminate the MySQL #340 foreign-key metadata hotspot'
status: completed
type: bug
priority: high
tags:
    - order-06
    - network-performance
    - mysql
    - foreign-keys
    - issue-340
    - performance
    - ready-for-agent
    - ticket-key-mysql-foreign-key-hotspot
created_at: 2026-09-12T03:19:44Z
updated_at: 2026-09-12T09:46:15Z
parent: lazysql-a9lf
blocked_by:
    - lazysql-846w
---

## What to build

Fix the MySQL foreign-key metadata lookup highlighted by issue #340 so moving FK loading to background does not merely hide an expensive multi-second catalog query.

Use the fastest compatible InnoDB metadata path when available, while preserving a safe fallback to `information_schema.KEY_COLUMN_USAGE` for permission, version, MariaDB, engine, or compatibility cases.

The operation must remain cancelable and preserve existing LazySQL FK metadata semantics.

## Acceptance criteria

- [x] MySQL FK metadata loading has a fast path based on compatible InnoDB metadata where available.
- [x] The fast path returns the FK information LazySQL needs with existing observable semantics preserved.
- [x] Permission or compatibility failure falls back to the existing compatible metadata mechanism rather than breaking FK functionality.
- [x] Servers where the fast path is unavailable remain supported through fallback.
- [x] FK metadata loading uses the caller context and can be cancelled at the database layer.
- [x] Fast-path and fallback behavior are covered by tests without requiring a privileged production server.
- [x] A regression/performance scenario representative of issue #340 proves that Records first paint is not blocked by FK metadata.
- [x] Background FK lookup no longer relies solely on the known pathological query path.
- [x] Debug logging can identify FK lookup duration and whether fallback was used.

## Summary of Changes

- Added MySQL 5.6/5.7 and 8.x InnoDB foreign-key dictionary fast paths with KEY_COLUMN_USAGE fallback.
- Propagated cancellable contexts through FK metadata loading and all driver implementations.
- Added fast-path, fallback, cancellation, and Records-first-paint regression coverage with structured debug timing logs.
