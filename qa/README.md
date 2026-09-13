# AI-driven end-to-end QA

This directory contains the first vertical slice of LazySQL's AI-driven smoke
harness. It is deliberately separate from the regular Go test suite and CI:
the smoke suite needs Docker, a real terminal multiplexer (Herdr), and a fresh
Pi session for every case.

## What is covered

| Case | Existing fixture connection | Goal |
| --- | --- | --- |
| `records-pagination` | `Docker MySQL` | Open the seeded `orders` table and prove that Records can move to a later page and back. |
| `foreign-key-jump` | `Docker PostgreSQL` | Follow the seeded `public.orders.customer_id` relationship to `public.customers.id`. |
| `sql-editor-truncation` | `Docker MySQL` | Stream a `SELECT` from the seeded `order_items` table and observe the configured 1,000-row interactive cap. |

The PostgreSQL connection is intentional. The current implementation enables
Foreign Key Jump for PostgreSQL, SQLite, and MSSQL, but not MySQL; the case must
therefore use a provider on the supported path rather than silently testing a
feature that MySQL does not render.

All three cases use the existing manual fixture stack and local config:

- `docker-compose.yml`
- `scripts/manual-databases.sh`
- `.lazysql.toml`
- `testdata/{mysql,postgres,mssql,sqlite}`

The runner starts that stack with `./scripts/manual-databases.sh up`; it does
not create a second Compose project, database image, seed script, or fixture.
For each case it resolves the named URL from this existing TOML and passes it
as LazySQL's positional connection argument, so the CLI `--read-only` flag is
effective without copying credentials into `suite.json`. The stack is left
available after a run so a preserved failed/blocked Herdr workspace can be
debugged. Use the existing `./scripts/manual-databases.sh
down` or `reset` commands when you explicitly want to stop or recreate it.

## Validate without an AI run

From the repository root:

```console
$ ./scripts/manual-databases.sh validate
$ ./scripts/qa-run.sh --validate
qa harness validation: PASS
$ ./scripts/qa-run.sh --dry-run
```

`--validate` checks the suite schema, the three case files, the existing
fixture/config files, the ignored artifact directory, and shell syntax. It does
not require Docker, Herdr, Pi, or a model. `--dry-run` emits a JSON execution
plan and performs no build, Docker, Herdr, or AI-agent operation. Filter the
plan (or a real run) with, for example:

```console
$ ./scripts/qa-run.sh --dry-run --case records-pagination
$ ./scripts/qa-run.sh --case foreign-key-jump
```

A real run must be started from a Herdr-managed pane (`HERDR_ENV=1`) and needs
`docker`, `go`, `jq`, `herdr`, and `pi` on `PATH`:

```console
$ ./scripts/qa-run.sh
```

Optional controls are documented by `./scripts/qa-run.sh --help`. A custom run
ID can be supplied with `--run-id`; IDs are kept in the artifact path and are
never reused by the runner.

## Ownership and lifecycle

The deterministic shell runner owns:

1. validating the suite and prerequisites;
2. starting the existing manual database fixtures;
3. building a run-local LazySQL binary;
4. creating artifact directories and recording runner/Herdr transcripts;
5. starting one isolated, named headless Herdr server for the run (never the
   user's default session);
6. creating one Herdr workspace for each case, with a LazySQL pane and a new
   Pi QA-agent pane;
7. applying timeouts and waiting for the result file; and
8. validating and aggregating the machine-readable results.

The Pi QA agent owns only human-like TUI navigation and semantic judgement. It
reads the case goal/oracles, observes the LazySQL terminal, sends terminal
input through Herdr, captures evidence, and writes the result contract. It
must not use a database client, query the database outside LazySQL, inspect
implementation code, or turn a fixed key sequence into the test.

A valid case result has one of `PASS`, `FAIL`, or `BLOCKED` as its status. A
passing case closes the Herdr workspace that the runner created. Failed and
blocked cases intentionally keep that workspace, its pane IDs, and the
run-local named Herdr session in the case artifacts for debugging. When every
case passes, the runner stops and deletes that isolated session after closing
all case workspaces. A missing, malformed, or timed-out agent result is
`BLOCKED`, not a false pass. The suite status is `FAIL` if any case fails,
otherwise `BLOCKED` if any case is blocked, otherwise `PASS`.

The runner exits `0` for `PASS`, `1` for `FAIL`, and `2` for `BLOCKED`.

## Artifacts

Every real run is stored under:

```text
.qa-runs/<run-id>/
  run.json
  suite.json
  runner.log
  herdr-server.stdout
  herdr-server.stderr
  herdr-config.toml
  herdr-status.json
  herdr-server.pid
  build/lazysql
  <case-id>/
    case-spec.json
    case.md
    workspace-create.json
    agent-pane.json
    lazysql-start.ansi
    lazysql-final.ansi
    lazysql.jsonl
    agent-final.ansi
    agent-result.json       # raw Pi output, when present
    result.json             # validated/enriched case result
    workspace-close.json    # PASS cases only
  suite-result.json
```

`.qa-runs/` is ignored. Do not commit credentials, raw connection URLs, or
unredacted terminal evidence. The supplied fixture is development-only and the
runner launches LazySQL in read-only mode. The run-local Herdr config/socket
uses a short dedicated namespace under `/tmp` (the config is copied into the
run artifacts); it is created and queried through an explicit named session so
the user's default Herdr workspace is never touched. A failed/blocked run
deliberately leaves that session available; clean it up with the recorded
session name after debugging.

## Source-checked TUI facts

The case instructions were written after checking the current implementation,
not by guessing labels:

- `app/keymap.go` maps Table `>`/`<` to `PageNext`/`PagePrev`, menu `1` to
  Records and `4` to Foreign Keys, Home `Ctrl-E` to the SQL editor, and Editor
  `Ctrl-R` to Execute.
- `components/home.go` creates the editor from the current database context;
  `components/results_table.go` handles Enter on a supported FK cell by opening
  the referenced table with a filter.
- `components/pagination.go` renders unknown-more, estimated, and exact forms;
  the cases therefore assert page movement semantically instead of requiring a
  particular count label.
- `components/results_table.go` and `drivers/query_stream.go` show that editor
  batches are rendered while a query is active, then a finite cap performs one
  lookahead. With `.lazysql.toml`'s `max_query_rows = 1000`, the terminal status
  is expected to communicate both `1000` rows shown and truncation.

The agent may consult the on-screen Help/keybindings and must adapt if a local
keymap changes. The source facts above are orientation for maintainers, not an
invitation to bypass the terminal-only test boundary.
