---
# lazysql-l2t6
title: '[02] Make database connection concurrency safe and configurable'
status: completed
type: task
priority: normal
tags:
    - order-02
    - ticket-key-configurable-connection-pool
    - network-performance
    - connections
    - configuration
    - pooling
    - ready-for-agent
created_at: 2026-09-12T03:19:43Z
updated_at: 2026-09-12T06:59:46Z
parent: lazysql-a9lf
---

## What to build

Add safe, explicit database connection-pool configuration so later background metadata and streaming work can use concurrency without opening an uncontrolled number of server connections.

Provide application defaults of 8 open / 8 idle connections for MySQL, PostgreSQL, and MSSQL, with optional per-connection overrides. Preserve SQLite correctness by forcing a single open/idle connection.

Zero values must use LazySQL defaults rather than Go's unlimited-open-connections semantics.

## Acceptance criteria

- [x] Application configuration exposes `max_open_connections` with default 8.
- [x] Application configuration exposes `max_idle_connections` with default 8.
- [x] Individual database connections may override both values.
- [x] Missing per-connection overrides inherit application-level values.
- [x] A configured value of 0 uses the LazySQL default and never means unlimited open connections.
- [x] Configuration rejects `max_idle_connections > max_open_connections` when both effective values are positive.
- [x] MySQL, PostgreSQL, and MSSQL apply the effective configured pool limits.
- [x] SQLite always uses one open and one idle connection regardless of server-driver defaults.
- [x] SQLite in-memory behavior remains correct under the pool configuration.
- [x] Configuration defaults and overrides have regression tests.


## Summary of Changes

- Added 8/8 application connection-pool defaults with optional per-connection overrides and effective-value validation.
- Applied safe pool limits to MySQL, PostgreSQL, and MSSQL, including PostgreSQL database-switch connections.
- Forced SQLite to one open/idle connection and added in-memory correctness coverage.
- Added configuration, driver, and documentation regression coverage.
