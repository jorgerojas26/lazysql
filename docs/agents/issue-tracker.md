# Issue tracker: Beans

Issues and specs for this repo live in **beans**, the agentic-first issue
tracker, managed through the `beans` CLI. Run `beans prime` for the current
usage guide.

## Conventions

- "Issues" are called **beans**. Each has a status, type, priority, body, and
  optional relationships.
- Types (`-t`, required on create): `milestone`, `epic`, `bug`, `feature`, `task`.
- Statuses: `draft` → `todo` → `in-progress` → `completed` | `scrapped`.
- Priorities (`-p`): `critical`, `high`, `normal` (default), `low`, `deferred`.
- Relationships: `--parent` (hierarchy), `--blocking`, `--blocked-by`. Prefer
  `--blocked-by` when creating dependent work.
- Checklist items live in the bean body (`- [ ]` / `- [x]`). A completed bean
  gets a `## Summary of Changes` section; a scrapped one gets
  `## Reasons for Scrapping`.
- Pass `--json` whenever a skill needs to parse output.
- Commit bean files together with the code changes they describe.

## When a skill says "publish to the issue tracker"

    beans create --json "Title" -t <type> -d "Description..." -s todo

Attach hierarchy with `--parent <id>`, dependencies with `--blocked-by <id>`.

## When a skill says "fetch the relevant ticket"

    beans show --json <id> [id...]

The user will normally pass the bean id directly. Discover candidates with
`beans list --json`, `beans list --json --ready`, or `beans list --json -S "<text>"`.

## When a skill says "update" or "close" a ticket

    beans update --json <id> -s in-progress
    beans update --json <id> -s completed --body-append "## Summary of Changes..."

Only mark a bean `completed` when it has no unchecked checklist items left.

## Wayfinding operations

Used by `/wayfinder`. The **map** is a parent bean; each **child ticket** is a
child bean.

- **Map**: parent bean created with `-t epic`.
- **Child ticket**: `beans create ... --parent <map-id>`, with a `Type:` line in
  the body recording `research` / `prototype` / `grilling` / `task`.
- **Blocking**: `--blocked-by` edges. A ticket is unblocked when every bean it
  is blocked by is `completed`.
- **Frontier**: open, unblocked, unclaimed children of the map — find with
  `beans list --json --ready`.
- **Claim**: `beans update <id> -s in-progress` before any work.
- **Resolve**: append the answer to the bean body, mark it `completed`, then
  append a context pointer (gist + bean id) to the map bean.