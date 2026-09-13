# Smoke: SQL-editor progressive results and truncation

- **Case ID:** `sql-editor-truncation`
- **Fixture connection:** `Docker MySQL`
- **Fixture table:** `order_items`
- **Configured cap:** `max_query_rows = 1000`
- **Mode:** read-only real LazySQL TUI

## Goal

Use the saved `Docker MySQL` connection from `.lazysql.toml`, open the SQL
editor, and run a read-only SELECT whose rows come from the seeded
`order_items` table. Observe that results arrive progressively and that the
interactive result is capped at 1,000 rows with an explicit truncation status.
The fixture contains 9,000 `order_items` rows.

The runner has already opened the named connection in read-only mode. Start
from the connected Home/tree screen. Open the editor from the connected home
view and enter this query through the editor itself:

```sql
SELECT id, order_id, product_id, quantity, SLEEP(0.001) AS stream_tick FROM order_items ORDER BY id;
```

`SLEEP(0.001)` is a small MySQL-only pacing expression so a local fast fixture
still leaves time to see progressive rendering. It is not the oracle and must
not be used through a database shell. If the selected connection is not MySQL,
or the expression produces an error, the case is BLOCKED rather than silently
changing the test to another environment.

## Oracles

Before executing, capture the editor state. Submit the query with the editor's
visible Execute action, then read the LazySQL pane repeatedly while the query is
active. Capture an intermediate screen that proves a non-empty result batch is
already rendered while the query is still running (for example, a running-query
or loading status alongside result rows). Do not wait only for the final state;
that would not prove progressive delivery.

After completion, capture the final result and verify semantically:

1. the result header/rows identify the selected `order_items` columns and show
   real seeded values;
2. the final status communicates `1000 rows shown`; and
3. the final status communicates `result truncated` and `maximum 1000` (wording
   and punctuation may be rendered differently, but all meanings must be
   visible).

PASS requires the intermediate non-empty batch and all final cap/truncation
observations. A result with more than the configured cap, a missing truncation
signal, a query error, or no visible progressive batch is FAIL when the TUI was
reachable. An unavailable fixture, wrong provider, or uncontrollable terminal
is BLOCKED.

## Implementation orientation (already reviewed)

`app/keymap.go` maps Home `Ctrl-E` to opening the editor and Editor `Ctrl-R` to
Execute. `components/sql_editor.go` starts the editor in insert mode, and
`components/results_table.go` keeps the query status while streamed batches are
painted. `drivers/query_stream.go` emits bounded batches and performs one
lookahead row at the finite cap. The current completion status is built from
the configured cap, so assert its meaning rather than a brittle full line.
Inspect the actual focus/help/status before sending input; do not replay a
blind key sequence.

## Evidence and judgement

Capture editor-before, intermediate-progress, and final-result ANSI screens in
the case artifact directory. Include relative paths in the observations. Never
put the database URL or credentials in evidence notes or result details.
