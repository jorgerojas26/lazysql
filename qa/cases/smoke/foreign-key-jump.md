# Smoke: Foreign Key Jump

- **Case ID:** `foreign-key-jump`
- **Fixture connection:** `Docker PostgreSQL`
- **Source table:** `public.orders`
- **Relationship:** `public.orders.customer_id` → `public.customers.id`
- **Mode:** read-only real LazySQL TUI

## Goal

Use the saved `Docker PostgreSQL` connection from `.lazysql.toml` and open the
seeded `public.orders` table. Prove through the UI that a populated
`customer_id` cell becomes a Foreign Key Jump after metadata arrives, and that
activating it opens the matching `public.customers` row with a filter.

The provider choice matters: the current implementation supports Foreign Key
Jump for PostgreSQL, SQLite, and MSSQL, while MySQL intentionally does not
advertise this action. Do not substitute the MySQL connection for this case.

The runner has already opened the named connection in read-only mode. Start
from the connected Home/tree screen. Expand the database/schema/tree nodes as
rendered and open `public.orders`. Wait for Records and structural metadata to
settle. If needed, inspect the Foreign Keys surface and return to Records using
the visible keybinding/help; do not infer readiness from a fixed sleep. Find a
non-empty `customer_id` value in an ordinary seeded row (not an edited/marked
row).

## Oracles

Capture the source table before and after metadata readiness. The UI should
show the Foreign Key Jump affordance for the `customer_id` header/cell (the
current renderer uses underline styling, which may be easier to confirm by
comparing the screen or the help/status context than by relying on color alone).

Select that populated `customer_id` cell and invoke the UI's ordinary Enter
action. PASS requires observing all of the following:

1. a new/current target view for `public.customers`, not merely the Foreign Keys
   metadata list;
2. the target is filtered to the value selected in the source cell; and
3. the visible target row has `id` equal to that selected customer ID.

The filter may be rendered as a field/query state rather than a single fixed
phrase. Judge the target table and matching row semantically. The relationship
is proven by the source value and target `id`, not by the presence of a table
named `customers` alone.

## Implementation orientation (already reviewed)

`app/keymap.go` maps Table `4` to the Foreign Keys surface and `1` to Records.
`components/results_table.go` handles a raw Enter on a supported, populated
foreign-key cell and calls `Home.ShowTableWithFilter` for the referenced table.
PostgreSQL metadata identifies the source column, referenced schema/table, and
referenced column; the renderer normalizes the target to `public.customers`.
These are orientation facts, not a fixed action script. Confirm the current
focus, rendered menu, and actual keybinding before each input.

## Evidence and judgement

Capture source Records with the navigable cell, the metadata/readiness state,
and the filtered target view as ANSI screen snapshots. Put relative paths in
the result observations. Report **FAIL** for a reachable UI that marks or
jumps incorrectly; report **BLOCKED** when the fixture, metadata, or terminal
cannot reach an oracle; report **PASS** only when the matching target row is
visible and supported by evidence.
