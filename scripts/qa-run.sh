#!/usr/bin/env bash
# Run the AI-driven LazySQL smoke suite in real Herdr-managed PTYs.
#
# The shell runner is intentionally deterministic: it owns fixture startup,
# builds, timeouts, artifacts, result validation, and aggregation. Pi owns only
# human-like terminal interaction and semantic judgement.
set -Eeuo pipefail

SCRIPT_PATH=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/$(basename -- "${BASH_SOURCE[0]}")
ROOT=$(CDPATH= cd -- "$(dirname -- "$SCRIPT_PATH")/.." && pwd)
SUITE_FILE="$ROOT/qa/suite.json"
CONTRACT_FILE="$ROOT/qa/agent-contract.md"
MANUAL_DATABASES="$ROOT/scripts/manual-databases.sh"

MODE=run
RUN_ID="${QA_RUN_ID:-}"
AGENT_MODEL="${QA_AGENT_MODEL:-}"
AGENT_THINKING="${QA_AGENT_THINKING:-}"
TUI_START_TIMEOUT_MS="${QA_TUI_START_TIMEOUT_MS:-}"
HERDR_START_TIMEOUT_MS="${QA_HERDR_START_TIMEOUT_MS:-}"
AGENT_START_TIMEOUT_MS="${QA_AGENT_START_TIMEOUT_MS:-}"
AGENT_TIMEOUT_MS="${QA_AGENT_TIMEOUT_MS:-}"
CASE_TIMEOUT_MS="${QA_CASE_TIMEOUT_MS:-}"
SELECTED_CASES=()

HERDR_BIN="${QA_HERDR_BIN:-herdr}"
GO_BIN="${QA_GO_BIN:-go}"
JQ_BIN="${QA_JQ_BIN:-jq}"

RUN_DIR=""
RUN_LOG=""
RUN_STARTED_AT=""
FIXTURE_STATUS="not-started"
HERDR_SESSION=""
HERDR_XDG=""
HERDR_RUNTIME=""
HERDR_CONFIG=""
HERDR_SOCKET=""
HERDR_SERVER_PID=""
CASE_RESULT_FILES=()
ALL_CASE_IDS=()
CASE_IDS=()
RUN_SUFFIX=""

usage() {
  cat <<'EOF'
Usage: ./scripts/qa-run.sh [options]

Modes that do not run Pi:
  --validate                    Validate suite/files/shell syntax only
  --dry-run                     Validate and print a JSON execution plan

Real-run options:
  --case CASE_ID                Run one case (repeatable; default: all three)
  --run-id ID                   Artifact ID (default: UTC timestamp plus PID)
  --tui-timeout-ms N            LazySQL startup wait (default: suite value)
  --herdr-start-timeout-ms N    Isolated Herdr server wait (default: suite value)
  --agent-start-timeout-ms N    Pi readiness wait (default: suite value)
  --agent-timeout-ms N          Pi result wait (default: suite value)
  --case-timeout-ms N           Overall per-case result deadline (default: suite value)
  --agent-model MODEL           Optional Pi model passed to Herdr
  --agent-thinking LEVEL        Optional Pi thinking level passed to Herdr
  -h, --help                    Show this help

Environment overrides use the QA_* names above. A real run must be launched
inside a Herdr-managed pane (HERDR_ENV=1) and requires Docker, Go, jq, Herdr,
and Pi. Failed or blocked case workspaces are intentionally preserved.
EOF
}

fail() {
  printf 'qa-run: %s\n' "$*" >&2
  exit 2
}

now_iso() {
  date -u '+%Y-%m-%dT%H:%M:%SZ'
}

is_positive_integer() {
  [[ "${1:-}" =~ ^[0-9]+$ ]] && [ "$1" -gt 0 ]
}

validate_run_id() {
  [[ "$1" =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$ ]]
}

require_command() {
  local command_name=$1
  if ! command -v "$command_name" >/dev/null 2>&1; then
    printf 'qa harness validation: missing command: %s\n' "$command_name" >&2
    return 1
  fi
}

validate_suite() {
  local failed=0
  local required_file
  local instruction
  local case_id
  local case_json
  local required_files=(
    "$SUITE_FILE"
    "$CONTRACT_FILE"
    "$MANUAL_DATABASES"
    "$ROOT/docker-compose.yml"
    "$ROOT/.lazysql.toml"
    "$ROOT/testdata/mysql/001-schema-and-data.sql"
    "$ROOT/testdata/postgres/001-schema-and-data.sql"
    "$ROOT/testdata/mssql/001-schema-and-data.sql"
    "$ROOT/testdata/sqlite/001-schema-and-data.sql"
  )

  if ! require_command "$JQ_BIN"; then
    failed=1
  fi

  for required_file in "${required_files[@]}"; do
    if [ ! -s "$required_file" ]; then
      printf 'qa harness validation: required file is missing or empty: %s\n' "$required_file" >&2
      failed=1
    fi
  done

  if [ "$failed" -ne 0 ]; then
    return 1
  fi

  if ! "$JQ_BIN" -e '
    (type == "object") and
    (.schema_version == 1) and
    (.name | type == "string") and
    (.fixture | type == "object") and
    (.fixture.reuse_existing == true) and
    (.fixture.startup == "./scripts/manual-databases.sh up") and
    (.fixture.config == ".lazysql.toml") and
    (.fixture.read_only == true) and
    (.defaults.page_size == 100) and
    (.defaults.max_query_rows == 1000) and
    (.defaults.herdr_start_timeout_ms == 30000) and
    (.cases | type == "array" and length == 3) and
    (([.cases[].id] | sort) == ["foreign-key-jump", "records-pagination", "sql-editor-truncation"]) and
    all(.cases[];
      (.id | type == "string") and
      (.title | type == "string") and
      (.instruction | type == "string") and
      (.connection | type == "string") and
      (.provider | type == "string") and
      (.oracles | type == "array" and length > 0) and
      (if .id == "records-pagination" then (.connection == "Docker MySQL" and .provider == "mysql" and .table == "orders")
       elif .id == "foreign-key-jump" then (.connection == "Docker PostgreSQL" and .provider == "postgres" and .table == "public.orders")
       elif .id == "sql-editor-truncation" then (.connection == "Docker MySQL" and .provider == "mysql" and .table == "order_items")
       else false end)
    )
  ' "$SUITE_FILE" >/dev/null; then
    printf 'qa harness validation: invalid qa/suite.json schema or smoke-case list\n' >&2
    failed=1
  fi

  if ! bash -n "$SCRIPT_PATH"; then
    printf 'qa harness validation: shell syntax check failed: %s\n' "$SCRIPT_PATH" >&2
    failed=1
  fi

  if [ ! -x "$MANUAL_DATABASES" ]; then
    printf 'qa harness validation: manual database helper is not executable: %s\n' "$MANUAL_DATABASES" >&2
    failed=1
  fi

  if ! grep -Fxq '.qa-runs/' "$ROOT/.gitignore"; then
    printf 'qa harness validation: .gitignore must contain .qa-runs/\n' >&2
    failed=1
  fi

  # These checks deliberately point at the existing manual fixture instead of
  # accepting a new Compose/config path. The QA harness must not grow a second
  # database test environment.
  for required_file in \
    'docker-compose.yml' \
    'scripts/manual-databases.sh' \
    '.lazysql.toml' \
    'testdata/mysql' \
    'testdata/postgres' \
    'testdata/mssql' \
    'testdata/sqlite'; do
    if [ ! -e "$ROOT/$required_file" ]; then
      printf 'qa harness validation: existing fixture path is missing: %s\n' "$ROOT/$required_file" >&2
      failed=1
    fi
  done

  while IFS= read -r case_id; do
    case_json=$("$JQ_BIN" -c --arg id "$case_id" '.cases[] | select(.id == $id)' "$SUITE_FILE")
    instruction=$(printf '%s\n' "$case_json" | "$JQ_BIN" -r '.instruction')
    case "$instruction" in
      /*|*..*)
        printf 'qa harness validation: case instruction must stay within the repository: %s\n' "$instruction" >&2
        failed=1
        ;;
      *)
        if [ ! -s "$ROOT/$instruction" ]; then
          printf 'qa harness validation: case instruction is missing or empty: %s\n' "$ROOT/$instruction" >&2
          failed=1
        fi
        ;;
    esac
  done < <("$JQ_BIN" -r '.cases[].id' "$SUITE_FILE")

  for required_file in \
    'Docker MySQL' \
    'Docker PostgreSQL' \
    'max_query_rows = 1000' \
    'mysql://lazysql:lazysql@127.0.0.1:3307/lazysql_test' \
    'postgres://lazysql:lazysql@127.0.0.1:5433/lazysql_test'; do
    if ! grep -Fq -- "$required_file" "$ROOT/.lazysql.toml"; then
      printf 'qa harness validation: expected existing fixture config is missing: %s\n' "$required_file" >&2
      failed=1
    fi
  done

  if [ "$failed" -ne 0 ]; then
    return 1
  fi
  return 0
}

set_suite_defaults() {
  if [ -z "$TUI_START_TIMEOUT_MS" ]; then
    TUI_START_TIMEOUT_MS=$("$JQ_BIN" -r '.defaults.tui_start_timeout_ms' "$SUITE_FILE")
  fi
  if [ -z "$HERDR_START_TIMEOUT_MS" ]; then
    HERDR_START_TIMEOUT_MS=$("$JQ_BIN" -r '.defaults.herdr_start_timeout_ms' "$SUITE_FILE")
  fi
  if [ -z "$AGENT_START_TIMEOUT_MS" ]; then
    AGENT_START_TIMEOUT_MS=$("$JQ_BIN" -r '.defaults.agent_start_timeout_ms' "$SUITE_FILE")
  fi
  if [ -z "$CASE_TIMEOUT_MS" ]; then
    CASE_TIMEOUT_MS=$("$JQ_BIN" -r '.defaults.case_timeout_ms' "$SUITE_FILE")
  fi
  if [ -z "$AGENT_TIMEOUT_MS" ]; then
    # Leave a small amount of the case budget for startup, snapshots, and
    # result validation while keeping the agent wait bounded by the case.
    AGENT_TIMEOUT_MS=$((CASE_TIMEOUT_MS * 9 / 10))
  fi

  if ! is_positive_integer "$TUI_START_TIMEOUT_MS"; then
    fail "tui startup timeout must be a positive integer (milliseconds)"
  fi
  if ! is_positive_integer "$HERDR_START_TIMEOUT_MS"; then
    fail "Herdr startup timeout must be a positive integer (milliseconds)"
  fi
  if ! is_positive_integer "$AGENT_START_TIMEOUT_MS"; then
    fail "agent startup timeout must be a positive integer (milliseconds)"
  fi
  if ! is_positive_integer "$AGENT_TIMEOUT_MS"; then
    fail "agent timeout must be a positive integer (milliseconds)"
  fi
  if ! is_positive_integer "$CASE_TIMEOUT_MS"; then
    fail "case timeout must be a positive integer (milliseconds)"
  fi
}

load_case_ids() {
  ALL_CASE_IDS=()
  while IFS= read -r case_id; do
    ALL_CASE_IDS+=("$case_id")
  done < <("$JQ_BIN" -r '.cases[].id' "$SUITE_FILE")

  if [ "${#SELECTED_CASES[@]}" -eq 0 ]; then
    CASE_IDS=("${ALL_CASE_IDS[@]}")
    return
  fi

  CASE_IDS=()
  local requested
  local found
  local existing
  for requested in "${SELECTED_CASES[@]}"; do
    found=0
    for existing in "${ALL_CASE_IDS[@]}"; do
      if [ "$requested" = "$existing" ]; then
        found=1
        break
      fi
    done
    if [ "$found" -eq 0 ]; then
      fail "unknown case ID: $requested"
    fi

    for existing in "${CASE_IDS[@]}"; do
      if [ "$requested" = "$existing" ]; then
        fail "case selected more than once: $requested"
      fi
    done
    CASE_IDS+=("$requested")
  done
}

selected_cases_json() {
  local selected='[]'
  local case_id
  for case_id in "${CASE_IDS[@]}"; do
    selected=$(printf '%s\n' "$selected" | "$JQ_BIN" --arg id "$case_id" '. + [$id]')
  done
  printf '%s' "$selected"
}

print_dry_run() {
  local selected_json
  local plan_json
  selected_json=$(selected_cases_json)
  plan_json=$("$JQ_BIN" -c --argjson selected "$selected_json" '
    [.cases[] | . as $case | select(($selected | index($case.id)) != null) |
      {id, title, instruction, connection, provider, table, oracles}]
  ' "$SUITE_FILE")

  "$JQ_BIN" -n \
    --arg run_id "$RUN_ID" \
    --arg artifact_root ".qa-runs/$RUN_ID" \
    --arg fixture "./scripts/manual-databases.sh up" \
    --arg config ".lazysql.toml" \
    --arg binary ".qa-runs/$RUN_ID/build/lazysql" \
    --argjson cases "$plan_json" \
    ' {
      schema_version: 1,
      mode: "dry-run",
      status: "DRY_RUN",
      run_id: $run_id,
      artifact_root: $artifact_root,
      would_invoke_ai: false,
      fixture: {reuse_existing: true, startup: $fixture, config: $config},
      build: {command: ("go build -trimpath -o " + $binary + " ."), output: $binary},
      herdr: {
        isolated_named_server: true,
        per_case: [
          "workspace create --cwd repository --no-focus",
          "pane split --direction right --no-focus",
          "pane run LazySQL in read-only mode",
          "agent start --kind pi --no-session",
          "agent prompt with --wait"
        ],
        successful_case_cleanup: ["workspace close", "session stop", "session delete"],
        failed_or_blocked_case: "preserve workspace and named session"
      },
      cases: $cases,
      aggregation: "FAIL > BLOCKED > PASS"
    }'
}

require_runtime() {
  if [ "${HERDR_ENV:-}" != "1" ]; then
    fail "real QA runs must be launched from a Herdr-managed pane (HERDR_ENV=1); use --validate or --dry-run outside Herdr"
  fi
  local command_name
  for command_name in "$HERDR_BIN" "$GO_BIN" "$JQ_BIN" docker pi; do
    if ! require_command "$command_name"; then
      fail "runtime prerequisite is unavailable: $command_name"
    fi
  done
}

shell_quote() {
  # Use POSIX single-quote escaping because Herdr's isolated panes use /bin/sh,
  # even though this runner itself is Bash.
  local value=${1//\'/\'\\\'\'}
  printf "'%s'" "$value"
}

# Resolve the named URL from the checked-in local TOML. The suite intentionally
# stores a connection name, not a second copy of fixture credentials. The
# format is constrained to the existing .lazysql.toml entries and this helper
# keeps the command-line URL out of runner logs and result JSON.
config_url_for_connection() {
  local wanted=$1
  awk -v wanted="$wanted" '
    function toml_value(line) {
      sub(/^[^=]+=[[:space:]]*/, "", line)
      sub(/[[:space:]]+$/, "", line)
      if (substr(line, 1, 1) == "\"" && substr(line, length(line), 1) == "\"") {
        line = substr(line, 2, length(line) - 2)
      }
      return line
    }
    /^\[\[database\]\]/ {
      in_database = 1
      name = ""
      next
    }
    in_database && /^[[:space:]]*Name[[:space:]]*=/ {
      name = toml_value($0)
      next
    }
    in_database && /^[[:space:]]*URL[[:space:]]*=/ && name == wanted {
      print toml_value($0)
      exit
    }
  ' "$ROOT/.lazysql.toml"
}

# Every real run gets its own headless Herdr server and socket. The parent
# agent may itself be inside Herdr, so inherited caller IDs and the default
# session must be stripped from every client request.
herdr_ctl() {
  env \
    -u HERDR_ENV \
    -u HERDR_SOCKET_PATH \
    -u HERDR_CLIENT_SOCKET_PATH \
    -u HERDR_PANE_ID \
    -u HERDR_TAB_ID \
    -u HERDR_WORKSPACE_ID \
    -u HERDR_SESSION \
    -u HERDR_CONFIG_PATH \
    XDG_CONFIG_HOME="$HERDR_XDG" \
    XDG_RUNTIME_DIR="$HERDR_RUNTIME" \
    HERDR_CONFIG_PATH="$HERDR_CONFIG" \
    "$HERDR_BIN" --session "$HERDR_SESSION" "$@"
}

start_herdr_server() {
  if [ -e "$HERDR_XDG" ] || [ -e "$HERDR_RUNTIME" ]; then
    printf '[%s] dedicated Herdr namespace already exists; refusing to reuse it\n' "$(now_iso)" >> "$RUN_LOG"
    return 1
  fi
  mkdir -p "$HERDR_XDG/herdr" "$HERDR_RUNTIME"
  chmod 700 "$HERDR_RUNTIME"
  cat > "$HERDR_CONFIG" <<'EOF'
onboarding = false

[terminal]
default_shell = "/bin/sh"
shell_mode = "non_login"
new_cwd = "follow"

[server]
headless_cols = 160
headless_rows = 50

[session]
resume_agents_on_restore = false
EOF
  cp "$HERDR_CONFIG" "$RUN_DIR/herdr-config.toml"

  printf '[%s] start isolated Herdr server %s\n' "$(now_iso)" "$HERDR_SESSION" >> "$RUN_LOG"
  nohup env \
    -u HERDR_ENV \
    -u HERDR_SOCKET_PATH \
    -u HERDR_CLIENT_SOCKET_PATH \
    -u HERDR_PANE_ID \
    -u HERDR_TAB_ID \
    -u HERDR_WORKSPACE_ID \
    -u HERDR_SESSION \
    -u HERDR_CONFIG_PATH \
    XDG_CONFIG_HOME="$HERDR_XDG" \
    XDG_RUNTIME_DIR="$HERDR_RUNTIME" \
    HERDR_CONFIG_PATH="$HERDR_CONFIG" \
    "$HERDR_BIN" --session "$HERDR_SESSION" server \
    > "$RUN_DIR/herdr-server.stdout" \
    2> "$RUN_DIR/herdr-server.stderr" \
    < /dev/null &
  HERDR_SERVER_PID=$!
  printf '%s\n' "$HERDR_SERVER_PID" > "$RUN_DIR/herdr-server.pid"
}

wait_for_herdr_server() {
  local deadline=$(( $(date +%s) + (HERDR_START_TIMEOUT_MS + 999) / 1000 ))
  local now
  local status_file="$RUN_DIR/herdr-status.json.tmp"

  while :; do
    if ! kill -0 "$HERDR_SERVER_PID" >/dev/null 2>&1; then
      printf '[%s] isolated Herdr server exited before readiness\n' "$(now_iso)" >> "$RUN_LOG"
      return 1
    fi
    if [ -S "$HERDR_SOCKET" ] && herdr_ctl status --json > "$status_file" 2>> "$RUN_LOG"; then
      mv -f "$status_file" "$RUN_DIR/herdr-status.json"
      return 0
    fi
    now=$(date +%s)
    if [ "$now" -ge "$deadline" ]; then
      printf '[%s] timed out waiting for isolated Herdr socket %s\n' "$(now_iso)" "$HERDR_SOCKET" >> "$RUN_LOG"
      return 1
    fi
    sleep 0.05
  done
}

stop_and_delete_herdr_session() {
  if ! capture_herdr "$RUN_DIR/herdr-session-stop.json" "stop isolated Herdr session $HERDR_SESSION" \
    session stop "$HERDR_SESSION" --json; then
    return 1
  fi
  if ! capture_herdr "$RUN_DIR/herdr-session-delete.json" "delete isolated Herdr session $HERDR_SESSION" \
    session delete "$HERDR_SESSION" --json; then
    return 1
  fi
  case "$(basename -- "$HERDR_XDG")" in
    lazysql-qa-xdg-*) ;;
    *)
      printf '[%s] refusing to remove an unrecognized Herdr namespace path\n' "$(now_iso)" >> "$RUN_LOG"
      return 1
      ;;
  esac
  if ! rm -rf "$HERDR_XDG" "$HERDR_RUNTIME"; then
    return 1
  fi
  return 0
}

run_logged() {
  local label=$1
  shift
  printf '[%s] %s\n' "$(now_iso)" "$label" >> "$RUN_LOG"
  if (CDPATH= cd -- "$ROOT" && "$@") >> "$RUN_LOG" 2>&1; then
    return 0
  else
    local rc=$?
    printf '[%s] %s failed with exit %s\n' "$(now_iso)" "$label" "$rc" >> "$RUN_LOG"
    return "$rc"
  fi
}

capture_herdr() {
  local output=$1
  local label=$2
  shift 2
  : > "$output"
  printf '[%s] %s\n' "$(now_iso)" "$label" >> "$RUN_LOG"
  if herdr_ctl "$@" > "$output" 2>> "$RUN_LOG"; then
    return 0
  else
    local rc=$?
    printf '[%s] %s failed with exit %s\n' "$(now_iso)" "$label" "$rc" >> "$RUN_LOG"
    return "$rc"
  fi
}

capture_pane() {
  local pane_id=$1
  local output=$2
  if capture_herdr "$output" "capture LazySQL pane $pane_id -> $(basename -- "$output")" \
    pane read --source visible --format ansi "$pane_id"; then
    return 0
  fi
  printf 'Herdr could not capture pane %s\n' "$pane_id" >> "$output"
  return 1
}

write_blocked_result() {
  local case_dir=$1
  local case_id=$2
  local reason=$3
  local phase=$4
  local workspace_id=${5:-}
  local lazy_pane=${6:-}
  local agent_name=${7:-}
  local preserved=${8:-true}
  local workspace_json='null'
  local lazy_json='null'
  local agent_json='null'
  local preserved_json=false
  local evidence_json='[]'
  local path
  local tmp="$case_dir/result.json.tmp"

  [ -n "$workspace_id" ] && workspace_json=$(printf '%s' "$workspace_id" | "$JQ_BIN" -R .)
  [ -n "$lazy_pane" ] && lazy_json=$(printf '%s' "$lazy_pane" | "$JQ_BIN" -R .)
  [ -n "$agent_name" ] && agent_json=$(printf '%s' "$agent_name" | "$JQ_BIN" -R .)
  [ "$preserved" = true ] && preserved_json=true

  for path in lazysql-start.ansi lazysql-final.ansi agent-final.ansi; do
    if [ -f "$case_dir/$path" ]; then
      evidence_json=$(printf '%s\n' "$evidence_json" | "$JQ_BIN" --arg path "$path" '. + [$path]')
    fi
  done

  "$JQ_BIN" -n \
    --arg case_id "$case_id" \
    --arg reason "$reason" \
    --arg phase "$phase" \
    --argjson workspace "$workspace_json" \
    --argjson lazy_pane "$lazy_json" \
    --argjson agent "$agent_json" \
    --argjson preserved "$preserved_json" \
    --argjson evidence "$evidence_json" \
    --arg started_at "$RUN_STARTED_AT" \
    --arg finished_at "$(now_iso)" \
    '{
      schema_version: 1,
      case_id: $case_id,
      status: "BLOCKED",
      summary: $reason,
      observations: [],
      evidence: $evidence,
      started_at: $started_at,
      finished_at: $finished_at,
      runner: {
        phase: $phase,
        workspace_id: $workspace,
        lazy_pane: $lazy_pane,
        agent: $agent,
        workspace_preserved: $preserved
      }
    }' > "$tmp"
  mv -f "$tmp" "$case_dir/result.json"
}

prepare_case_dir() {
  local case_id=$1
  local case_dir="$RUN_DIR/$case_id"
  local case_json
  local instruction

  mkdir -p "$case_dir"
  case_json=$("$JQ_BIN" -c --arg id "$case_id" '.cases[] | select(.id == $id)' "$SUITE_FILE")
  printf '%s\n' "$case_json" | "$JQ_BIN" . > "$case_dir/case-spec.json"
  instruction=$(printf '%s\n' "$case_json" | "$JQ_BIN" -r '.instruction')
  cp "$ROOT/$instruction" "$case_dir/case.md"
}

block_selected_cases() {
  local case_id
  local case_dir
  for case_id in "${CASE_IDS[@]}"; do
    prepare_case_dir "$case_id"
    case_dir="$RUN_DIR/$case_id"
    write_blocked_result "$case_dir" "$case_id" "$1" "$2" "" "" "" true
    CASE_RESULT_FILES+=("$case_dir/result.json")
  done
}

wait_for_file() {
  local file=$1
  local deadline_epoch=$2
  local now
  while [ ! -s "$file" ]; do
    now=$(date +%s)
    if [ "$now" -ge "$deadline_epoch" ]; then
      return 1
    fi
    sleep 1
  done
  return 0
}

validate_agent_result() {
  local result_file=$1
  local case_id=$2
  local case_dir=$3
  local evidence_paths
  local evidence

  if ! "$JQ_BIN" -e --arg case_id "$case_id" '
    (type == "object") and
    (.schema_version == 1) and
    (.case_id == $case_id) and
    (.status == "PASS" or .status == "FAIL" or .status == "BLOCKED") and
    (.summary | type == "string" and length > 0) and
    (.observations | type == "array" and length > 0) and
    (.evidence | type == "array") and
    all(.evidence[]; type == "string" and length > 0) and
    (.started_at | type == "string") and
    (.finished_at | type == "string") and
    all(.observations[];
      (type == "object") and
      (.id | type == "string" and length > 0) and
      (.status == "PASS" or .status == "FAIL" or .status == "BLOCKED") and
      (.details | type == "string" and length > 0) and
      (.evidence | type == "array") and
      all(.evidence[]; type == "string" and length > 0)
    ) and
    (if .status == "PASS" then all(.observations[]; .status == "PASS") else true end)
  ' "$result_file" >/dev/null; then
    return 1
  fi

  if ! evidence_paths=$("$JQ_BIN" -r '(.evidence[]?, .observations[]?.evidence[]?) | select(type == "string")' "$result_file"); then
    return 1
  fi
  while IFS= read -r evidence; do
    [ -n "$evidence" ] || continue
    case "$evidence" in
      /*|../*|*/../*|*"/.."|*".."*)
        return 1
        ;;
    esac
    if [ ! -f "$case_dir/$evidence" ]; then
      return 1
    fi
  done <<< "$evidence_paths"
  return 0
}

enrich_agent_result() {
  local result_file=$1
  local case_dir=$2
  local case_id=$3
  local workspace_id=$4
  local lazy_pane=$5
  local agent_name=$6
  local cleanup_state=$7
  local workspace_preserved=$8
  local preserved_json=false
  local workspace_json='null'
  local lazy_json='null'
  local agent_json='null'
  local tmp="$case_dir/result.json.tmp"

  [ "$workspace_preserved" = true ] && preserved_json=true
  [ -n "$workspace_id" ] && workspace_json=$(printf '%s' "$workspace_id" | "$JQ_BIN" -R .)
  [ -n "$lazy_pane" ] && lazy_json=$(printf '%s' "$lazy_pane" | "$JQ_BIN" -R .)
  [ -n "$agent_name" ] && agent_json=$(printf '%s' "$agent_name" | "$JQ_BIN" -R .)

  "$JQ_BIN" \
    --arg case_id "$case_id" \
    --arg phase "agent-judgement" \
    --arg cleanup "$cleanup_state" \
    --argjson workspace "$workspace_json" \
    --argjson lazy_pane "$lazy_json" \
    --argjson agent "$agent_json" \
    --argjson preserved "$preserved_json" \
    '.schema_version = 1 |
     .case_id = $case_id |
     .evidence = ((.evidence + ["lazysql-start.ansi", "lazysql-final.ansi", "agent-final.ansi"]) | unique) |
     .runner = {
       phase: $phase,
       cleanup: $cleanup,
       workspace_id: $workspace,
       lazy_pane: $lazy_pane,
       agent: $agent,
       workspace_preserved: $preserved
     }' "$result_file" > "$tmp"
  mv -f "$tmp" "$case_dir/result.json"
}

run_case() {
  local case_id=$1
  local case_dir="$RUN_DIR/$case_id"
  local case_json
  local instruction
  local connection_name
  local provider
  local connection_url
  local workspace_id=''
  local tab_id=''
  local lazy_pane=''
  local agent_pane=''
  local agent_name=''
  local agent_result="$case_dir/agent-result.json"
  local agent_prompt_ok=1
  local agent_status=''
  local cleanup_state='preserved'
  local workspace_preserved=true
  local case_deadline
  local tui_command
  local case_prompt
  local agent_start_args=()

  prepare_case_dir "$case_id"
  case_deadline=$(( $(date +%s) + (CASE_TIMEOUT_MS + 999) / 1000 ))
  case_json=$("$JQ_BIN" -c --arg id "$case_id" '.cases[] | select(.id == $id)' "$SUITE_FILE")
  instruction=$(printf '%s\n' "$case_json" | "$JQ_BIN" -r '.instruction')
  connection_name=$(printf '%s\n' "$case_json" | "$JQ_BIN" -r '.connection')
  provider=$(printf '%s\n' "$case_json" | "$JQ_BIN" -r '.provider')
  connection_url=$(config_url_for_connection "$connection_name")
  if [ -z "$connection_url" ]; then
    write_blocked_result "$case_dir" "$case_id" "The named fixture connection has no URL in .lazysql.toml" "config-lookup" "" "" "" true
    CASE_RESULT_FILES+=("$case_dir/result.json")
    return 0
  fi

  if ! capture_herdr "$case_dir/workspace-create.json" "create Herdr workspace for $case_id" \
    workspace create --cwd "$ROOT" --label "lazysql-qa-$case_id" --no-focus; then
    write_blocked_result "$case_dir" "$case_id" "Herdr could not create the case workspace" "workspace-create" "" "" "" true
    CASE_RESULT_FILES+=("$case_dir/result.json")
    return 0
  fi

  workspace_id=$("$JQ_BIN" -r '.result.workspace.workspace_id // empty' "$case_dir/workspace-create.json" 2>/dev/null || true)
  tab_id=$("$JQ_BIN" -r '.result.tab.tab_id // empty' "$case_dir/workspace-create.json" 2>/dev/null || true)
  lazy_pane=$("$JQ_BIN" -r '.result.root_pane.pane_id // empty' "$case_dir/workspace-create.json" 2>/dev/null || true)
  if [ -z "$workspace_id" ] || [ -z "$lazy_pane" ]; then
    write_blocked_result "$case_dir" "$case_id" "Herdr returned no usable workspace or LazySQL pane ID" "workspace-create" "$workspace_id" "$lazy_pane" "" true
    CASE_RESULT_FILES+=("$case_dir/result.json")
    return 0
  fi

  # Naming is diagnostic only; all later operations use IDs returned by Herdr.
  run_logged "rename case tab $tab_id" herdr_ctl tab rename "$tab_id" "qa-$case_id" || true
  run_logged "rename LazySQL pane $lazy_pane" herdr_ctl pane rename "$lazy_pane" lazysql || true

  if ! capture_herdr "$case_dir/agent-pane.json" "split fresh QA-agent pane for $case_id" \
    pane split --pane "$lazy_pane" --direction right --ratio 0.50 --cwd "$ROOT" --no-focus \
    --env "QA_CASE_ID=$case_id" \
    --env "QA_CONNECTION_NAME=$connection_name" \
    --env "QA_PROVIDER=$provider" \
    --env "QA_WORKSPACE_ID=$workspace_id" \
    --env "QA_LAZYSQL_PANE=$lazy_pane" \
    --env "QA_ARTIFACT_DIR=$case_dir" \
    --env "QA_AGENT_RESULT=$agent_result"; then
    capture_pane "$lazy_pane" "$case_dir/lazysql-final.ansi" || true
    write_blocked_result "$case_dir" "$case_id" "Herdr could not split the fresh QA-agent pane" "pane-split" "$workspace_id" "$lazy_pane" "" true
    CASE_RESULT_FILES+=("$case_dir/result.json")
    return 0
  fi

  agent_pane=$("$JQ_BIN" -r '.result.pane.pane_id // empty' "$case_dir/agent-pane.json" 2>/dev/null || true)
  if [ -z "$agent_pane" ]; then
    capture_pane "$lazy_pane" "$case_dir/lazysql-final.ansi" || true
    write_blocked_result "$case_dir" "$case_id" "Herdr returned no fresh QA-agent pane ID" "pane-split" "$workspace_id" "$lazy_pane" "" true
    CASE_RESULT_FILES+=("$case_dir/result.json")
    return 0
  fi
  run_logged "rename QA-agent pane $agent_pane" herdr_ctl pane rename "$agent_pane" qa-agent || true

  tui_command="env $(shell_quote "QA_RUN_ID=$RUN_ID") $(shell_quote "QA_CASE_ID=$case_id") $(shell_quote "QA_ARTIFACT_DIR=$case_dir") $(shell_quote "$RUN_DIR/build/lazysql") --loglevel debug --logfile $(shell_quote "$case_dir/lazysql.jsonl") --read-only $(shell_quote "$connection_url")"
  if ! capture_herdr "$case_dir/lazysql-start.json" "start real LazySQL TUI for $case_id" \
    pane run "$lazy_pane" "$tui_command"; then
    capture_pane "$lazy_pane" "$case_dir/lazysql-final.ansi" || true
    write_blocked_result "$case_dir" "$case_id" "Herdr could not start the real LazySQL TUI" "lazysql-start" "$workspace_id" "$lazy_pane" "" true
    CASE_RESULT_FILES+=("$case_dir/result.json")
    return 0
  fi

  # The URL came from the existing local config. Passing it to InitFromArg
  # makes the CLI's --read-only flag effective while the agent still performs
  # all database/tree/table navigation through the real TUI.
  if ! capture_herdr "$case_dir/lazysql-ready.json" "wait for connected LazySQL tree" \
    pane wait-output --source visible --regex 'Databases|Search:' \
    --timeout "$TUI_START_TIMEOUT_MS" "$lazy_pane"; then
    printf '[%s] LazySQL readiness text was not observed; delegating screen judgement to Pi\n' "$(now_iso)" >> "$RUN_LOG"
  fi
  capture_pane "$lazy_pane" "$case_dir/lazysql-start.ansi" || true

  local agent_suffix
  agent_suffix=$(printf '%s' "$RUN_ID" | tr -cd '[:alnum:]' | tr '[:upper:]' '[:lower:]' | tail -c 4)
  [ -n "$agent_suffix" ] || agent_suffix="$$"
  agent_name="qa-$case_id-$agent_suffix"

  # Herdr rejects control characters in forwarded agent arguments. Pi accepts
  # an append-system-prompt file path, so pass the contract by path instead of
  # embedding its multiline contents in the agent-start request.
  agent_start_args=(--no-session --no-context-files --name "$agent_name" --append-system-prompt "$CONTRACT_FILE")
  if [ -n "$AGENT_MODEL" ]; then
    agent_start_args+=(--model "$AGENT_MODEL")
  fi
  if [ -n "$AGENT_THINKING" ]; then
    agent_start_args+=(--thinking "$AGENT_THINKING")
  fi

  if ! capture_herdr "$case_dir/agent-start.json" "start fresh Pi QA agent $agent_name" \
    agent start "$agent_name" --kind pi --pane "$agent_pane" --timeout "$AGENT_START_TIMEOUT_MS" -- \
    "${agent_start_args[@]}"; then
    capture_pane "$lazy_pane" "$case_dir/lazysql-final.ansi" || true
    capture_herdr "$case_dir/agent-final.ansi" "capture failed Pi startup" \
      agent read "$agent_name" --source recent-unwrapped --format ansi || true
    write_blocked_result "$case_dir" "$case_id" "Fresh Pi QA agent did not become ready" "agent-start" "$workspace_id" "$lazy_pane" "$agent_name" true
    CASE_RESULT_FILES+=("$case_dir/result.json")
    return 0
  fi
  capture_herdr "$case_dir/agent-session.json" "record fresh Pi agent identity" agent get "$agent_name" || true

  case_prompt=$(cat <<EOF
Run exactly one LazySQL smoke case: $case_id.

Read the case instructions at $ROOT/$instruction before acting. The runner
has opened the saved "$connection_name" fixture connection (provider:
$provider) in read-only mode. Start from the connected LazySQL home screen; do
not assume that focus, tree expansion, or rendered wording is unchanged.
Interact with LazySQL only through the Herdr terminal pane "$lazy_pane" using
screen reads and terminal input, as required by the QA contract.

Use these supplied locations:
  QA_CASE_ID=$case_id
  QA_WORKSPACE_ID=$workspace_id
  QA_LAZYSQL_PANE=$lazy_pane
  QA_ARTIFACT_DIR=$case_dir
  QA_AGENT_RESULT=$agent_result

Capture evidence into the artifact directory and write the required JSON to
QA_AGENT_RESULT when all goal/oracle checks are complete. Do not merely report
back in chat. Use PASS only for directly observed success, FAIL for an
observed product regression, and BLOCKED when the environment or interaction
prevents a semantic judgement. Never use a database client, Docker, direct SQL
connection, source inspection, or a second terminal path to infer the result.
EOF
)

  if ! capture_herdr "$case_dir/agent-prompt.json" "give $agent_name the $case_id goal" \
    agent prompt "$agent_name" "$case_prompt" --wait --timeout "$AGENT_TIMEOUT_MS"; then
    agent_prompt_ok=0
    printf '[%s] Pi prompt did not settle within its timeout; result file will decide status\n' "$(now_iso)" >> "$RUN_LOG"
  fi

  if ! wait_for_file "$agent_result" "$case_deadline"; then
    capture_pane "$lazy_pane" "$case_dir/lazysql-final.ansi" || true
    capture_herdr "$case_dir/agent-final.ansi" "capture timed-out Pi agent" \
      agent read "$agent_name" --source recent-unwrapped --format ansi || true
    if [ "$agent_prompt_ok" -eq 0 ]; then
      write_blocked_result "$case_dir" "$case_id" "Pi QA agent timed out or did not produce a result JSON" "agent-timeout" "$workspace_id" "$lazy_pane" "$agent_name" true
    else
      write_blocked_result "$case_dir" "$case_id" "Pi QA agent finished without producing a result JSON" "agent-result" "$workspace_id" "$lazy_pane" "$agent_name" true
    fi
    CASE_RESULT_FILES+=("$case_dir/result.json")
    return 0
  fi

  capture_pane "$lazy_pane" "$case_dir/lazysql-final.ansi" || true
  capture_herdr "$case_dir/agent-final.ansi" "capture settled Pi agent" \
    agent read "$agent_name" --source recent-unwrapped --format ansi || true

  if ! validate_agent_result "$agent_result" "$case_id" "$case_dir"; then
    write_blocked_result "$case_dir" "$case_id" "Pi result JSON failed the machine-readable contract or referenced missing evidence" "result-validation" "$workspace_id" "$lazy_pane" "$agent_name" true
    CASE_RESULT_FILES+=("$case_dir/result.json")
    return 0
  fi

  agent_status=$("$JQ_BIN" -r '.status' "$agent_result")
  if [ "$agent_status" = PASS ]; then
    if capture_herdr "$case_dir/workspace-close.json" "close successful case workspace $workspace_id" \
      workspace close "$workspace_id"; then
      cleanup_state=closed
      workspace_preserved=false
      enrich_agent_result "$agent_result" "$case_dir" "$case_id" "$workspace_id" "$lazy_pane" "$agent_name" "$cleanup_state" "$workspace_preserved"
    else
      write_blocked_result "$case_dir" "$case_id" "Pi reported PASS but the runner could not clean up its Herdr workspace" "workspace-cleanup" "$workspace_id" "$lazy_pane" "$agent_name" true
    fi
  else
    # FAIL and BLOCKED workspaces remain live for debugging, with IDs recorded
    # in result.json and raw screen/agent transcripts in this case directory.
    enrich_agent_result "$agent_result" "$case_dir" "$case_id" "$workspace_id" "$lazy_pane" "$agent_name" "$cleanup_state" "$workspace_preserved"
  fi

  CASE_RESULT_FILES+=("$case_dir/result.json")
  return 0
}

aggregate_suite() {
  local finished_at
  local tmp="$RUN_DIR/suite-result.json.tmp"
  local status

  finished_at=$(now_iso)
  "$JQ_BIN" -s \
    --arg run_id "$RUN_ID" \
    --arg started_at "$RUN_STARTED_AT" \
    --arg finished_at "$finished_at" \
    --arg fixture_status "$FIXTURE_STATUS" \
    --arg artifacts ".qa-runs/$RUN_ID" \
    --arg herdr_session "$HERDR_SESSION" \
    --arg herdr_namespace "$HERDR_XDG" \
    'def overall:
       if any(.[]; .status == "FAIL") then "FAIL"
       elif any(.[]; .status == "BLOCKED") then "BLOCKED"
       else "PASS" end;
     {
       schema_version: 1,
       run_id: $run_id,
       status: overall,
       started_at: $started_at,
       finished_at: $finished_at,
       fixture_status: $fixture_status,
       artifacts: $artifacts,
       herdr_session: $herdr_session,
       herdr_namespace: $herdr_namespace,
       cases: .
     }' "${CASE_RESULT_FILES[@]}" > "$tmp"
  mv -f "$tmp" "$RUN_DIR/suite-result.json"
  status=$("$JQ_BIN" -r '.status' "$RUN_DIR/suite-result.json")

  case "$status" in
    PASS) return 0 ;;
    FAIL) return 1 ;;
    BLOCKED) return 2 ;;
    *) printf 'qa-run: aggregator emitted invalid status: %s\n' "$status" >&2; return 2 ;;
  esac
}

finish_suite() {
  local aggregate_rc
  local final_status
  local tmp="$RUN_DIR/suite-result.json.tmp"

  if aggregate_suite; then
    aggregate_rc=0
  else
    aggregate_rc=$?
  fi

  if [ "$aggregate_rc" -eq 0 ]; then
    if stop_and_delete_herdr_session; then
      "$JQ_BIN" '. + {herdr_session_cleanup: "deleted"}' "$RUN_DIR/suite-result.json" > "$tmp"
      mv -f "$tmp" "$RUN_DIR/suite-result.json"
    else
      # All semantic checks passed, but infrastructure cleanup did not. Keep
      # the isolated server available and make the machine result conservative.
      "$JQ_BIN" '.status = "BLOCKED" | .herdr_session_cleanup = "preserved_cleanup_failed"' "$RUN_DIR/suite-result.json" > "$tmp"
      mv -f "$tmp" "$RUN_DIR/suite-result.json"
      aggregate_rc=2
    fi
  else
    "$JQ_BIN" --arg cleanup "preserved" '. + {herdr_session_cleanup: $cleanup}' "$RUN_DIR/suite-result.json" > "$tmp"
    mv -f "$tmp" "$RUN_DIR/suite-result.json"
  fi

  final_status=$("$JQ_BIN" -r '.status' "$RUN_DIR/suite-result.json")
  "$JQ_BIN" -c . "$RUN_DIR/suite-result.json"
  case "$final_status" in
    PASS) return 0 ;;
    FAIL) return 1 ;;
    BLOCKED) return 2 ;;
    *) printf 'qa-run: final suite result has invalid status: %s\n' "$final_status" >&2; return 2 ;;
  esac
}

write_run_metadata() {
  local selected_json
  selected_json=$(selected_cases_json)
  "$JQ_BIN" -n \
    --arg run_id "$RUN_ID" \
    --arg started_at "$RUN_STARTED_AT" \
    --arg branch "$(git -C "$ROOT" branch --show-current 2>/dev/null || printf 'unknown')" \
    --arg herdr_session "$HERDR_SESSION" \
    --argjson cases "$selected_json" \
    ' {
      schema_version: 1,
      run_id: $run_id,
      started_at: $started_at,
      branch: $branch,
      cases: $cases,
      herdr: {isolated: true, session: $herdr_session, server: "headless", namespace: "run-local-short-path"},
      fixture: {reuse_existing: true, startup: "./scripts/manual-databases.sh up", config: ".lazysql.toml"},
      read_only: true
    }' > "$RUN_DIR/run.json"
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      --validate)
        [ "$MODE" = run ] || fail "choose only one of --validate and --dry-run"
        MODE=validate
        ;;
      --dry-run)
        [ "$MODE" = run ] || fail "choose only one of --validate and --dry-run"
        MODE=dry-run
        ;;
      --case)
        [ "$#" -ge 2 ] || fail "--case requires a case ID"
        SELECTED_CASES+=("$2")
        shift
        ;;
      --run-id)
        [ "$#" -ge 2 ] || fail "--run-id requires an ID"
        RUN_ID=$2
        shift
        ;;
      --tui-timeout-ms)
        [ "$#" -ge 2 ] || fail "--tui-timeout-ms requires a number"
        TUI_START_TIMEOUT_MS=$2
        shift
        ;;
      --herdr-start-timeout-ms)
        [ "$#" -ge 2 ] || fail "--herdr-start-timeout-ms requires a number"
        HERDR_START_TIMEOUT_MS=$2
        shift
        ;;
      --agent-start-timeout-ms)
        [ "$#" -ge 2 ] || fail "--agent-start-timeout-ms requires a number"
        AGENT_START_TIMEOUT_MS=$2
        shift
        ;;
      --agent-timeout-ms)
        [ "$#" -ge 2 ] || fail "--agent-timeout-ms requires a number"
        AGENT_TIMEOUT_MS=$2
        shift
        ;;
      --case-timeout-ms)
        [ "$#" -ge 2 ] || fail "--case-timeout-ms requires a number"
        CASE_TIMEOUT_MS=$2
        shift
        ;;
      --agent-model)
        [ "$#" -ge 2 ] || fail "--agent-model requires a model ID"
        AGENT_MODEL=$2
        shift
        ;;
      --agent-thinking)
        [ "$#" -ge 2 ] || fail "--agent-thinking requires a level"
        AGENT_THINKING=$2
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        fail "unknown option: $1"
        ;;
    esac
    shift
  done
}

main() {
  parse_args "$@"

  case "$MODE" in
    validate|dry-run)
      if ! validate_suite; then
        if [ "$MODE" = validate ]; then
          exit 1
        fi
        exit 1
      fi
      set_suite_defaults
      if [ -z "$RUN_ID" ]; then
        RUN_ID="$(date -u '+%Y%m%dT%H%M%SZ')-$$"
      fi
      if ! validate_run_id "$RUN_ID"; then
        fail "run ID must match [A-Za-z0-9][A-Za-z0-9._-]{0,63}"
      fi
      load_case_ids
      if [ "$MODE" = validate ]; then
        printf 'qa harness validation: PASS\n'
      else
        print_dry_run
      fi
      return 0
      ;;
    run)
      ;;
    *)
      fail "internal error: unknown mode $MODE"
      ;;
  esac

  if ! validate_suite; then
    fail "validation failed; use --validate for details"
  fi
  set_suite_defaults
  require_runtime

  if [ -z "$RUN_ID" ]; then
    RUN_ID="$(date -u '+%Y%m%dT%H%M%SZ')-$$"
  fi
  if ! validate_run_id "$RUN_ID"; then
    fail "run ID must match [A-Za-z0-9][A-Za-z0-9._-]{0,63}"
  fi
  load_case_ids

  RUN_DIR="$ROOT/.qa-runs/$RUN_ID"
  if [ -e "$RUN_DIR" ]; then
    fail "artifact directory already exists; choose a new --run-id: $RUN_DIR"
  fi
  mkdir -p "$RUN_DIR/build"
  RUN_LOG="$RUN_DIR/runner.log"
  : > "$RUN_LOG"
  RUN_STARTED_AT=$(now_iso)
  RUN_SUFFIX=$(printf '%s' "$RUN_ID" | tr -cd '[:alnum:]' | tr '[:upper:]' '[:lower:]' | tail -c 8)
  [ -n "$RUN_SUFFIX" ] || RUN_SUFFIX="$$"
  # Unix-domain sockets have a small path limit. Keep the dedicated Herdr
  # namespace under /tmp with a short per-run token, while copying its config
  # into the run artifacts. This avoids length-dependent startup failures.
  HERDR_SESSION="q-$RUN_SUFFIX"
  HERDR_XDG="${QA_HERDR_ROOT:-/tmp}/lazysql-qa-xdg-$RUN_SUFFIX"
  HERDR_RUNTIME="${QA_HERDR_ROOT:-/tmp}/lazysql-qa-runtime-$RUN_SUFFIX"
  HERDR_CONFIG="$HERDR_XDG/herdr/config.toml"
  HERDR_SOCKET="$HERDR_XDG/herdr/sessions/$HERDR_SESSION/herdr.sock"
  write_run_metadata
  cp "$SUITE_FILE" "$RUN_DIR/suite.json"

  if ! start_herdr_server || ! wait_for_herdr_server; then
    FIXTURE_STATUS=herdr-start-failed
    block_selected_cases "isolated Herdr server did not become ready" "herdr-start"
    if finish_suite; then
      return 0
    else
      local herdr_rc=$?
      return "$herdr_rc"
    fi
  fi

  if ! run_logged "build run-local LazySQL binary" "$GO_BIN" build -trimpath -o "$RUN_DIR/build/lazysql" .; then
    FIXTURE_STATUS=not-started
    block_selected_cases "LazySQL build failed; no AI case was started" "build"
    if finish_suite; then
      return 0
    else
      local build_rc=$?
      return "$build_rc"
    fi
  fi

  if ! run_logged "start existing manual database fixture stack" "$MANUAL_DATABASES" up; then
    FIXTURE_STATUS=failed
    block_selected_cases "existing manual database fixture startup failed" "fixture-startup"
    if finish_suite; then
      return 0
    else
      local fixture_rc=$?
      return "$fixture_rc"
    fi
  fi
  FIXTURE_STATUS=started

  local case_id
  for case_id in "${CASE_IDS[@]}"; do
    run_case "$case_id"
  done

  if finish_suite; then
    return 0
  else
    local suite_rc=$?
    return "$suite_rc"
  fi
}

main "$@"
