# LazySQL smoke QA-agent contract

This contract is supplied to every fresh Pi process by `scripts/qa-run.sh`.
The Pi process is a QA observer, not a coding agent. It must complete exactly
one case and must not reuse another case's session or workspace.

## Inputs

The runner starts the agent in a separate Herdr pane and supplies these
variables:

| Variable | Meaning |
| --- | --- |
| `QA_CASE_ID` | The case ID in `qa/suite.json`. |
| `QA_CONNECTION_NAME` | The saved connection name that the runner opened for this case. |
| `QA_PROVIDER` | The fixture provider for this case. |
| `QA_WORKSPACE_ID` | Herdr workspace containing both panes. |
| `QA_LAZYSQL_PANE` | The pane running the real LazySQL TUI; this is the only application terminal to control. |
| `QA_ARTIFACT_DIR` | The case artifact directory. |
| `QA_AGENT_RESULT` | Absolute path where the raw result JSON must be written. |

The runner also gives the agent the case instruction path. Read that Markdown
file for the goal, oracles, and evidence suggestions before acting.

## Terminal-only boundary

Interact with LazySQL exactly as a human tester would:

- Observe the current screen with
  `herdr pane read "$QA_LAZYSQL_PANE" --source visible --format ansi`.
- Send special keys with
  `herdr pane send-keys "$QA_LAZYSQL_PANE" ctrl+e`, `enter`, `esc`, and so
  on. Herdr uses names such as `ctrl+e` for control keys, validates them, and
  writes them to the real PTY.
- Send literal text with
  `herdr pane send-text "$QA_LAZYSQL_PANE" "..."`. This writes literal text
  to the PTY without appending Enter; send a separate `enter` key when the
  focused LazySQL widget needs submission.
- Re-read the screen after every meaningful transition. Wait for visible
  progress or completion rather than relying on a long, fixed sleep.

The agent may use its shell only to run Herdr terminal-control/read commands,
create evidence files, and atomically write `QA_AGENT_RESULT`. It must not:

- run `mysql`, `psql`, `sqlite3`, `sqlcmd`, Docker, or another database client;
- connect to the fixture or execute SQL anywhere except the LazySQL editor;
- inspect Go source, call application APIs, use an alternate TUI driver, or
  infer a result from implementation details;
- edit repository files, the local config, fixture data, or the database; or
- send input directly to a process/PTY by a tool other than Herdr.

Use the visible UI and the case's goal/oracles. Do not turn the orientation
keys in a case file into a blind hard-coded sequence: inspect the screen, use
Help when useful, and adapt to the actual focus and rendered labels. A local
keymap or a changed layout is evidence to reason about, not a reason to guess.

## Evidence

Capture the relevant screens into `QA_ARTIFACT_DIR` while the state is visible,
for example:

```console
$ herdr pane read "$QA_LAZYSQL_PANE" --source visible --format ansi \
    > "$QA_ARTIFACT_DIR/screen-initial.ansi"
```

Only put paths relative to `QA_ARTIFACT_DIR` in the result. The runner adds its
own start/final snapshots, validates every named evidence file, and preserves
raw agent output as `agent-result.json`. Do not put passwords or connection
URLs in notes or result details.

## Result file

Write one JSON object, with no Markdown fence and no prose, to
`QA_AGENT_RESULT`. Write it atomically (for example, write a temporary file in
the same directory and then `mv` it). The schema is:

```json
{
  "schema_version": 1,
  "case_id": "records-pagination",
  "status": "PASS",
  "summary": "The first and later Records pages were both visible and returned to the same first row.",
  "observations": [
    {
      "id": "later-page",
      "status": "PASS",
      "details": "The selected order changed after PageNext and the later page loaded without an error.",
      "evidence": ["screen-first.ansi", "screen-later.ansi"]
    }
  ],
  "evidence": ["screen-first.ansi", "screen-later.ansi"],
  "started_at": "2026-01-01T00:00:00Z",
  "finished_at": "2026-01-01T00:00:03Z"
}
```

`status` and every observation `status` must be exactly `PASS`, `FAIL`, or
`BLOCKED`.

- `PASS` means every oracle in the case was directly observed in the TUI and
  the evidence supports it.
- `FAIL` means the TUI was reachable and an oracle was observably wrong. State
  the failed oracle and evidence; do not fail because the UI wording differs
  from an assumption.
- `BLOCKED` means the environment or interaction could not reach an oracle
  (for example, the fixture was unavailable, the connection was not present,
  or the terminal could not be controlled). Explain what prevented judgement.

Do not write `PASS` for a timeout, missing row, unconfirmed transition, or
inaccessible screen. Do not omit an oracle just to make the status pass.
