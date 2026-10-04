#!/usr/bin/env bash
# Owner-only: read private stats without putting the token in shell history
# or the curl command line. Requires macOS Keychain, curl, and this account.
set -euo pipefail

url='https://lazysql-telemetry.jorgeluisrojasb.workers.dev/stats'
secret="$(security find-generic-password -a jorgerojas26 -s lazysql-telemetry-stats -w)"
if [[ ${#secret} -lt 32 ]]; then
  echo 'Missing/invalid stats token in Keychain' >&2
  exit 1
fi
printf 'header = "Authorization: Bearer %s"\n' "$secret" | curl --silent --show-error --fail --config - "$url"
printf '\n'
