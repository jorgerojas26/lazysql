#!/bin/sh
set -eu

sqlite3 /fixture/lazysql.sqlite3 < /fixture/001-schema-and-data.sql
# The app may update this fixture from the host. Keep it writable when the
# bind-mounted directory is created by a root-owned container.
chmod 666 /fixture/lazysql.sqlite3 2>/dev/null || true

exec tail -f /dev/null
