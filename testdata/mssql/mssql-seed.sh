#!/usr/bin/env bash
set -euo pipefail

sqlcmd=(/opt/mssql-tools18/bin/sqlcmd -S tcp:mssql,1433 -U sa -P "$MSSQL_SA_PASSWORD" -C -b -l 5)

# SELECT 1 can succeed before existing user databases finish recovering. Never
# interpret a failed metadata lookup as an empty database and destroy manual
# edits by reseeding it. Retry readiness, then seed only on a successful zero.
seed_ready=""
for attempt in {1..30}; do
  if result=$("${sqlcmd[@]}" -h -1 -Q "SET NOCOUNT ON; IF DB_ID(N'lazysql_test') IS NULL SELECT 0; ELSE IF CONVERT(nvarchar(60), DATABASEPROPERTYEX(N'lazysql_test', N'Status')) <> N'ONLINE' RAISERROR('Fixture database is recovering', 16, 1); ELSE IF OBJECT_ID(N'lazysql_test.dbo.fixture_seed_metadata', N'U') IS NOT NULL SELECT 1 ELSE SELECT 0" 2>&1); then
    seed_ready=$(printf '%s' "$result" | tr -d '[:space:]')
    break
  fi
  if [[ "$attempt" == "30" ]]; then
    printf 'Cannot determine fixture readiness; refusing to reseed: %s\n' "$result" >&2
    exit 1
  fi
  sleep 2
done

case "$seed_ready" in
  0) "${sqlcmd[@]}" -i /seed/001-schema-and-data.sql ;;
  1) ;; # Preserve existing fixture data.
  *) printf 'Unexpected fixture readiness response: %s\n' "$seed_ready" >&2; exit 1 ;;
esac

exec tail -f /dev/null
