#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

compose() {
  docker compose -f docker-compose.yml "$@"
}

usage() {
  cat <<'EOF'
Usage: ./scripts/manual-databases.sh <command>

Commands:
  up        Build the SQLite helper and start every fixture, waiting for health
  down      Stop fixtures while preserving database volumes and SQLite data
  reset     Destroy all fixture data and recreate the seeded environment
  status    Show container health and seed-service status
  validate  Validate the Compose file and required fixture/config files
EOF
}

up() {
  compose up -d --build --wait
}

case "${1:-}" in
  up)
    up
    ;;
  down)
    compose down --remove-orphans
    ;;
  reset)
    compose down --volumes --remove-orphans
    rm -f testdata/sqlite/*.sqlite3 testdata/sqlite/*.sqlite3-*
    up
    ;;
  status)
    compose ps
    ;;
  validate)
    compose config --quiet
    for file in \
      .lazysql.toml \
      docker/Dockerfile.sqlite \
      docker/sqlite-entrypoint.sh \
      testdata/mysql/001-schema-and-data.sql \
      testdata/postgres/001-schema-and-data.sql \
      testdata/mssql/001-schema-and-data.sql \
      testdata/mssql/mssql-seed.sh \
      testdata/sqlite/001-schema-and-data.sql; do
      test -s "$file" || {
        printf 'required fixture file is missing or empty: %s\n' "$file" >&2
        exit 1
      }
    done

    expect() {
      file=$1
      text=$2
      grep -Fq -- "$text" "$file" || {
        printf 'expected fixture value is missing: %s <- %s\n' "$file" "$text" >&2
        exit 1
      }
    }

    for provider in mysql postgres sqlserver sqlite3; do
      expect .lazysql.toml "Provider = \"$provider\""
    done
    expect docker-compose.yml '"3307:3306"'
    expect docker-compose.yml '"5433:5432"'
    expect docker-compose.yml '"14331:1433"'
    expect docker-compose.yml 'MYSQL_DATABASE: lazysql_test'
    expect docker-compose.yml 'POSTGRES_DB: lazysql_test'
    expect docker-compose.yml 'MSSQL_SA_PASSWORD: "Lazysql!Test123"'
    expect .lazysql.toml 'mysql://lazysql:lazysql@127.0.0.1:3307/lazysql_test'
    expect .lazysql.toml 'postgres://lazysql:lazysql@127.0.0.1:5433/lazysql_test'
    expect .lazysql.toml 'sqlserver://sa:Lazysql%21Test123@127.0.0.1:14331'
    expect .lazysql.toml 'file:./testdata/sqlite/lazysql.sqlite3'
    expect .lazysql.toml '_pragma=foreign_keys(1)'
    expect .lazysql.toml 'DBName = "lazysql_test"'
    expect testdata/README.md '3,000 orders'

    printf '%s\n' 'manual database environment is valid'
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
