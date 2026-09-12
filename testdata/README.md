# LazySQL manual fixtures

The fixture stack uses the same logical model in every supported provider:

- `customers` — 1,200 customers with nullable fields, dates, numeric balances,
  and JSON/text preferences.
- `products` — 300 products with unique SKUs, prices, categories, and active
  flags.
- `orders` — 3,000 orders linked to customers.
- `order_items` — three product lines per order (9,000 rows) with foreign keys,
  numeric quantities, and discounts.
- `customer_notes` — 2,400 longer text/JSON records linked to customers.
- `order_summary` — a view joining orders, customers, and item aggregates.
- `fixture_seed_metadata` — seed marker/version used by readiness checks and
  the MSSQL initializer.

All scripts include primary keys, foreign keys, unique constraints, indexes,
nulls, date/time values, numeric values, booleans, JSON/text values, and enough
rows to exercise the default pagination page and CSV exports.

The SQL is deliberately provider-specific so each driver also gets realistic
catalog metadata. SQLite's current tree lists its tables; query
`order_summary` from the SQL editor when you want to exercise the view. The
`sqlite/001-schema-and-data.sql` script is safe to run repeatedly;
server fixtures are initialized by their official images on an empty named
volume. Use `./scripts/manual-databases.sh reset` to recreate every fixture.
