# Smoke: Records pagination

- **Case ID:** `records-pagination`
- **Fixture connection:** `Docker MySQL`
- **Fixture table:** `orders`
- **Mode:** read-only real LazySQL TUI

## Goal

Use the saved `Docker MySQL` connection from the repository's `.lazysql.toml`,
open the seeded `orders` table, and prove that the Records surface can move to
a later page and return to the first page. The fixture contains 3,000 orders,
so it is deliberately larger than the configured 100-row page.

The runner has already opened the named connection in read-only mode. Start
from the connected Home/tree screen; do not create or edit a connection.
Navigate the database tree and open `orders`. Wait for the Records rows and the
loading indicator to settle before judging the first page. If the tree presents
a database node or schema node first, expand the visible node instead of
assuming a tree depth. For a stable fixture order, if the selected cell is the
`id` column, invoke the visible Sort ascending action and wait for that refresh
to finish before recording the first row.

## Oracles

Record the first visible order identity (the `id` cell is a useful stable
observation) and capture an initial screen. Then, while the Records surface has
focus:

1. invoke the currently displayed PageNext action (`>` in the reviewed
   default Table map) and wait for the next fetch to finish;
2. capture the later page and verify that it contains rows and a different
   order identity/page range, with no error state; and
3. invoke PagePrev (`<` in the reviewed default Table map), wait for the fetch
   to finish, and verify that the original first-page identity is visible
   again.

PASS requires all three observations. A pagination label may be unknown-more,
estimated, or exact depending on when the asynchronous count finishes; judge
the page transition and rows, not one hard-coded label. Do not press exact count
just to manufacture a particular label. If the implementation does not expose a
usable Sort ascending action or the page never settles, report BLOCKED rather
than assuming database physical row order.

## Implementation orientation (already reviewed)

The current source maps Table `>` to `PageNext` and `<` to `PagePrev` in
`app/keymap.go`. `components/home.go` performs those actions only for the
Records surface when it is not loading. The same Table map maps `K` to Sort
ascending. The Records page uses the configured `DefaultPageSize = 100` and a
lookahead row. Confirm focus and the actual Help or status text on screen before
sending a key; do not replay a fixed sequence blindly.

## Evidence and judgement

Capture at least the initial page, later page, and returned page as ANSI screen
snapshots in the case artifact directory. Include those relative filenames in
the result observations. Report:

- **FAIL** if LazySQL is reachable but a page is empty, unchanged when the
  transition should occur, erroneous, or cannot return to the first row;
- **BLOCKED** if the fixture/connection/tree cannot be reached or the terminal
  cannot establish the focus needed for a judgement; and
- **PASS** only when every oracle is directly visible in the TUI.
