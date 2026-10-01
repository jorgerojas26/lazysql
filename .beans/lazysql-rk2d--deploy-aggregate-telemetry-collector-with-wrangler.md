---
# lazysql-rk2d
title: Deploy aggregate telemetry collector with Wrangler
status: completed
type: task
priority: normal
created_at: 2026-10-01T02:06:25Z
updated_at: 2026-10-01T02:10:44Z
---

- [x] Check Wrangler account, deployment config and pending migrations
- [x] Apply aggregate D1 migration and deploy Worker
- [x] Verify live ingestion, authenticated stats and disabled logging

## Summary of Changes

Applied 0002_metrics.sql to the existing remote lazysql-telemetry D1 database with Wrangler 4.136.1 and deployed the schema-2 aggregate Worker. Deployment version: 47123f84-4e0e-49b3-bf49-8742e78165dd, serving 100% traffic at https://lazysql-telemetry.jorgeluisrojasb.workers.dev. Hourly cleanup and existing STATS_TOKEN secret preserved. No migrations remain pending.

Verified 13 local collector tests, live schema-2 heartbeat HTTP 204, extra-field rejection HTTP 400, unauthenticated stats HTTP 401, authenticated feature and heartbeat increments, five-minute cadence/ten-minute estimate window, and no response cookie. Removed only the smoke-test increments and confirmed aggregate counts returned to the zero baseline. Verified remote script-settings reports observability null (disabled) and Logpush false. Credentials were used only in memory, never printed or passed as command arguments. No application release was published.
