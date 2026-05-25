# Lazysql

Context language for interactive database exploration in terminal UI.

## Language

**Foreign Key Jump**:
Keyboard action that follows a foreign key value from a source row cell to a referenced table view with a prefilled filter.
_Avoid_: FK drilldown, relation follow, link jump

## Relationships

- A **Foreign Key Jump** starts from one selected table cell in a source row.
- A **Foreign Key Jump** targets one referenced table view filtered by the selected value.

## Example dialogue

> **Dev:** "Should Enter open JSON or follow relation here?"
> **Domain expert:** "If this cell supports **Foreign Key Jump**, Enter follows relation; otherwise Enter keeps existing behavior."

## Flagged ambiguities

- "drilldown" was used to mean both JSON inspection and FK navigation - resolved: use **Foreign Key Jump** only for relation navigation.
