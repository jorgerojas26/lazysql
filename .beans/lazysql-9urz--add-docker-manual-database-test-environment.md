---
# lazysql-9urz
title: Add Docker manual database test environment
status: completed
type: feature
priority: high
created_at: 2026-09-12T21:11:38Z
updated_at: 2026-09-12T22:01:36Z
---

# Manual database test environment

Provide a plug-and-play Docker Compose stack for manually exercising every database provider currently supported by LazySQL, with a checked-in local `.lazysql.toml` and deterministic seed data. The environment must be safe for local development, discoverable from concise documentation, and usable without editing credentials or connection URLs.

## Acceptance Criteria

- [x] Docker Compose starts isolated MySQL, PostgreSQL, MSSQL, and SQLite fixtures with documented readiness and teardown commands.
- [x] A project-root `.lazysql.toml` lists one working connection for every supported provider and uses the Compose service ports/credentials.
- [x] Seed scripts create representative relational schemas, constraints, indexes, varied data types, and enough rows to exercise browsing, filtering, sorting, pagination, metadata, and exports.
- [x] SQLite fixture is reproducibly created/populated without requiring a database container.
- [x] Initialization is idempotent or documented as clean-volume setup, and the stack does not depend on host-installed database clients.
- [x] Documentation explains quick start, selecting the local config, health checks, reset, and troubleshooting/architecture caveats.
- [x] Relevant validation (Compose config, seed/config consistency, and repository tests) passes.

## Summary of Changes

Implemented a Docker Compose manual fixture stack for MySQL 8.4, PostgreSQL 16, SQL Server 2022, and a bind-mounted SQLite helper. Added provider-specific seed scripts with 1,200 customers, 300 products, 3,000 orders, 9,000 order items, 2,400 notes, relationships, indexes, views, JSON/text, nullable, numeric, date/time, and boolean values. Added the root `.lazysql.toml`, lifecycle/validation helper, SQLite image, generated-file ignores, fixture documentation, and the README quick-start/troubleshooting section.

Validation passed with `docker compose config --quiet`, `./scripts/manual-databases.sh validate`, shell syntax checks, live MySQL/PostgreSQL/SQLite counts, a live MSSQL seed/count run on the available SQL Server image before the host ARM engine later crashed under unsupported emulation, `go test ./... -count=1`, and `go vet ./...`. The documented MSSQL caveat directs Apple Silicon users to native x86 or a remote SQL Server.
