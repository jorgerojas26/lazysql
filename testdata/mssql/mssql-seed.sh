#!/usr/bin/env bash
set -euo pipefail

sqlcmd=(/opt/mssql-tools18/bin/sqlcmd -S tcp:mssql,1433 -U sa -P "$MSSQL_SA_PASSWORD" -C -b -l 5)

# Keep manual edits when the seed container is restarted. A clean `reset`
# removes the named MSSQL volume and causes the SQL file to run again.
seed_ready="$("${sqlcmd[@]}" -h -1 -Q "SET NOCOUNT ON; IF OBJECT_ID(N'lazysql_test.dbo.fixture_seed_metadata', N'U') IS NOT NULL SELECT 1 ELSE SELECT 0" 2>/dev/null | tr -d '[:space:]' || true)"
if [[ "$seed_ready" != "1" ]]; then
  "${sqlcmd[@]}" -i /seed/001-schema-and-data.sql
fi

exec tail -f /dev/null
