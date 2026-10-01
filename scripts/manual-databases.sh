#!/usr/bin/env sh
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT"

compose() {
  if [ -f .env.manual-databases ]; then
    docker compose --env-file .env.manual-databases -f docker-compose.yml "$@"
  else
    docker compose -f docker-compose.yml "$@"
  fi
}

usage() {
  cat <<'EOF'
Usage: ./scripts/manual-databases.sh <command>

Commands:
  init      Generate private local credentials and .lazysql.toml (requires openssl)
  up        Build the SQLite helper and start every fixture, waiting for health
  down      Stop fixtures while preserving database volumes and SQLite data
  reset     Destroy all fixture data and recreate the seeded environment
  status    Show container health and seed-service status
  validate  Validate the Compose file and required fixture/config files
EOF
}

init() {
  if [ -e .env.manual-databases ] || [ -e .lazysql.toml ]; then
    printf '%s\n' 'Refusing to overwrite local credentials or .lazysql.toml' >&2
    exit 1
  fi
  # Alphanumeric password is URL-safe and meets SQL Server complexity rules.
  password="Aa1$(openssl rand -hex 24)"
  umask 077
  printf 'LAZYSQL_FIXTURE_PASSWORD=%s\n' "$password" > .env.manual-databases
  sed "s/@@PASSWORD@@/$password/g" .lazysql.example.toml > .lazysql.toml
  printf '%s\n' 'Generated local fixture credentials (git-ignored, mode 600)'
}

up() {
  if [ ! -f .env.manual-databases ]; then init; fi
  compose up -d --build --wait
}

case "${1:-}" in
  init)
    init
    ;;
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
    # Validate templates without writing credentials or requiring existing setup.
    LAZYSQL_FIXTURE_PASSWORD="Aa1$(openssl rand -hex 24)"
    export LAZYSQL_FIXTURE_PASSWORD
    compose config --quiet
    for file in \
      .lazysql.example.toml \
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
      expect .lazysql.example.toml "Provider = \"$provider\""
    done
    expect docker-compose.yml '"127.0.0.1:3307:3306"'
    expect docker-compose.yml '"127.0.0.1:5433:5432"'
    expect docker-compose.yml '"127.0.0.1:14331:1433"'
    expect docker-compose.yml 'MYSQL_DATABASE: lazysql_test'
    expect docker-compose.yml 'POSTGRES_DB: lazysql_test'
    expect .lazysql.example.toml 'mysql://lazysql:@@PASSWORD@@@127.0.0.1:3307/lazysql_test'
    expect .lazysql.example.toml 'postgres://lazysql:@@PASSWORD@@@127.0.0.1:5433/lazysql_test'
    expect .lazysql.example.toml 'sqlserver://sa:@@PASSWORD@@@127.0.0.1:14331'
    expect .lazysql.example.toml 'file:./testdata/sqlite/lazysql.sqlite3'
    expect .lazysql.example.toml '_pragma=foreign_keys(1)'
    expect .lazysql.example.toml 'DBName = "lazysql_test"'
    expect testdata/README.md '3,000 orders'

    printf '%s\n' 'manual database environment is valid'
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
