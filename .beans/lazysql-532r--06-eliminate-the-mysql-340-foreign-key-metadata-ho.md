---
# lazysql-532r
title: '[06] Eliminate the MySQL #340 foreign-key metadata hotspot'
status: todo
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
updated_at: 2026-09-12T03:19:45Z
parent: lazysql-a9lf
blocked_by:
    - lazysql-846w
---

## What to build

Fix the MySQL foreign-key metadata lookup highlighted by issue #340 so moving FK loading to background does not merely hide an expensive multi-second catalog query.

Use the fastest compatible InnoDB metadata path when available, while preserving a safe fallback to `information_schema.KEY_COLUMN_USAGE` for permission, version, MariaDB, engine, or compatibility cases.

The operation must remain cancelable and preserve existing LazySQL FK metadata semantics.

## Acceptance criteria

- [ ] MySQL FK metadata loading has a fast path based on compatible InnoDB metadata where available.
- [ ] The fast path returns the FK information LazySQL needs with existing observable semantics preserved.
- [ ] Permission or compatibility failure falls back to the existing compatible metadata mechanism rather than breaking FK functionality.
- [ ] Servers where the fast path is unavailable remain supported through fallback.
- [ ] FK metadata loading uses the caller context and can be cancelled at the database layer.
- [ ] Fast-path and fallback behavior are covered by tests without requiring a privileged production server.
- [ ] A regression/performance scenario representative of issue #340 proves that Records first paint is not blocked by FK metadata.
- [ ] Background FK lookup no longer relies solely on the known pathological query path.
- [ ] Debug logging can identify FK lookup duration and whether fallback was used.
