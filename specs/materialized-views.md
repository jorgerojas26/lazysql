# Spec: Support for Materialized Views

Issue: https://github.com/jorgerojas26/lazysql/issues/298

## Problem Statement

A PostgreSQL user cannot find their materialized views in the lazysql tree panel. The current `GetViews` implementation queries `information_schema.views`, which excludes materialized views. There is no mechanism to list or inspect materialized views at all.

## Solution

Add materialized view support to the tree panel. Materialized views appear in their own "materialized_views" section alongside existing "tables", "views", "functions", and "procedures" sections. Selecting a materialized view loads its SQL definition into the editor, mirroring the existing regular view behavior.

## User Stories

1. As a PostgreSQL user, I want to see my materialized views listed in the database tree, so that I can find and interact with them without leaving the TUI.
2. As a PostgreSQL user, I want materialized views separated from regular views in the tree (under "materialized_views"), so that I can distinguish between updatable views and snapshot materializations.
3. As a PostgreSQL user, I want to click a materialized view and see its definition SQL in the editor, so that I can review and modify the query that populates it.
4. As a PostgreSQL user, I want to browse the data in a materialized view by selecting it, so that I can inspect the cached results.
5. As a MySQL user, I want materialized views ignored gracefully, so that the tree still loads without errors for databases that don't support them.
6. As a SQLite user, I want materialized views ignored gracefully, so that the tree still loads without errors for databases that don't support them.
7. As an MSSQL user, I may eventually want indexed views supported, but this is a lower priority.

## Implementation Decisions

1. **New Driver interface methods**: Add `GetMaterializedViews(database string) (map[string][]string, error)` and `GetMaterializedViewDefinition(database, name string) (string, error)` to the `Driver` interface, following the exact same signature pattern as `GetViews` / `GetViewDefinition`.

2. **PostgreSQL implementation**: 
   - `GetMaterializedViews` queries `pg_catalog.pg_matviews` joined with `pg_catalog.pg_namespace` to return schema-qualified names (`schema.name`).
   - `GetMaterializedViewDefinition` queries `pg_catalog.pg_matviews` for the `definition` column.

3. **Other drivers**: MySQL, SQLite return `errors.New("not implemented")`. MSSQL could query `sys.objects` with `type_desc = 'VIEW'` and check `is_indexed` but is deferred — return `errors.New("not implemented")` for now.

4. **New tree node type**: `NodeTypeMaterializedView` (integer constant) added to the `TreeNodeType` enum in `components/tree.go`.

5. **Tree section label**: `"materialized_views"` in the tree — a separate top-level section, not nested under `"views"`.

6. **Tree reference format**: Follow existing pattern: `database.schema.materialized_views.name` (4 parts with schemas, 3 without).

7. **New event constant**: `eventTreeSelectedMaterializedView = "SelectedMaterializedView"` in `components/constants.go`.

8. **Tree building**: `buildSchemaTree` and `addProgrammingNodes` both get a `materializedViews` parameter mirroring `views`. Section added after `"views"` section.

9. **Editor loading**: When `eventTreeSelectedMaterializedView` fires, the home handler calls `GetMaterializedViewDefinition` and loads the result into the editor, identical to the existing view/func/proc flow.

10. **Data browsing**: Selecting a materialized view navigates to it like a table (its data is queryable via `SELECT * FROM`). This requires the `showTable` path to be invoked for `NodeTypeMaterializedView`, same as `NodeTypeTable`.

## Testing Decisions

- **Seam**: `Driver.GetMaterializedViews()` and `Driver.GetMaterializedViewDefinition()` are the primary test seams (interface methods).
- **Seam**: `buildSchemaTree` already has tests for views section — extend to cover materialized_views.
- **Seam**: `GetTreeNodeData` switch/case already well-covered — extend for `NodeTypeMaterializedView`.
- **Good test**: Verify the tree renders the materialized_views section with correct items given a mock driver returning known materialized view names. Do not test SQL query correctness (integration concern).
- **Prior art**: `components/tree_test.go` tests `buildSchemaTree` with views, functions, and procedures — extend with materialized views.

## Out of Scope

- Refreshing a materialized view (`REFRESH MATERIALIZED VIEW`) from the UI — can be done via the SQL editor.
- MSSQL indexed view support.
- Any write operations on materialized views (INSERT/UPDATE/DELETE already handled by the existing record editing flow since materialized views are queryable like tables).
- Distinguishing materialized views from regular views at the `GetTables` level — the `information_schema.tables` query currently returns all table-typed objects; filtering views out of the tables list is a separate concern.

## Further Notes

- The PostgreSQL `pg_matviews` catalog exists since PostgreSQL 9.3+. No version check needed for any supported version.
- Materialized views have data (unlike regular views), so browsing their records via `SELECT * FROM` works identically to tables — the existing `FetchRecords` path handles this naturally.
- The existing `GetTables` queries `information_schema.tables` which includes materialized views as they have `table_type = 'BASE TABLE'` (in some PG versions) or similar. This means materialized views may already appear in the "tables" section. Once materialized views are filtered into their own section, those appearing in the tables section become a deduplication concern — deferred to a follow-up issue.
