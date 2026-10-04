-- LazySQL manual fixture for SQLite.
-- This file is intentionally repeatable: the Docker helper uses INSERT OR IGNORE.

PRAGMA foreign_keys = ON;

BEGIN;

CREATE TABLE IF NOT EXISTS customers (
  id INTEGER PRIMARY KEY,
  external_id TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  email TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL CHECK (status IN ('active', 'trial', 'paused', 'closed')),
  signup_date TEXT NOT NULL,
  balance NUMERIC NOT NULL,
  preferences TEXT NOT NULL,
  phone TEXT,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS products (
  id INTEGER PRIMARY KEY,
  sku TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  category TEXT NOT NULL,
  unit_price NUMERIC NOT NULL,
  active INTEGER NOT NULL,
  attributes TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS orders (
  id INTEGER PRIMARY KEY,
  customer_id INTEGER NOT NULL REFERENCES customers (id),
  status TEXT NOT NULL CHECK (status IN ('pending', 'paid', 'shipped', 'cancelled', 'refunded')),
  ordered_at TEXT NOT NULL,
  total_amount NUMERIC NOT NULL,
  shipping_address TEXT NOT NULL,
  notes TEXT
);

CREATE TABLE IF NOT EXISTS order_items (
  id INTEGER PRIMARY KEY,
  order_id INTEGER NOT NULL REFERENCES orders (id),
  product_id INTEGER NOT NULL REFERENCES products (id),
  quantity INTEGER NOT NULL CHECK (quantity > 0),
  unit_price NUMERIC NOT NULL,
  discount_percent NUMERIC NOT NULL,
  UNIQUE (order_id, product_id)
);

CREATE TABLE IF NOT EXISTS customer_notes (
  id INTEGER PRIMARY KEY,
  customer_id INTEGER NOT NULL REFERENCES customers (id),
  note_text TEXT NOT NULL,
  metadata TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_orders_customer ON orders (customer_id);
CREATE INDEX IF NOT EXISTS idx_orders_status_ordered_at ON orders (status, ordered_at);
CREATE INDEX IF NOT EXISTS idx_order_items_product ON order_items (product_id);
CREATE INDEX IF NOT EXISTS idx_customer_notes_customer_created ON customer_notes (customer_id, created_at);
CREATE INDEX IF NOT EXISTS idx_products_category_active ON products (category, active);

WITH RECURSIVE numbers(n) AS (
  SELECT 1
  UNION ALL
  SELECT n + 1 FROM numbers WHERE n < 1200
)
INSERT OR IGNORE INTO customers
  (id, external_id, name, email, status, signup_date, balance, preferences, phone, created_at)
SELECT
  n,
  printf('CUS-%06d', n),
  printf('Customer %04d', n),
  'customer' || n || '@example.test',
  CASE n % 4 WHEN 0 THEN 'active' WHEN 1 THEN 'trial' WHEN 2 THEN 'paused' ELSE 'closed' END,
  date('2022-01-01', printf('+%d days', (n * 13) % 730)),
  round(25.50 + ((n * 37) % 800000) / 100.0, 2),
  printf('{"tier":"%s","marketing":%s}',
    CASE n % 3 WHEN 0 THEN 'gold' WHEN 1 THEN 'silver' ELSE 'standard' END,
    CASE WHEN n % 3 = 0 THEN 'true' ELSE 'false' END),
  CASE WHEN n % 5 = 0 THEN NULL ELSE printf('+1-555-%04d', n % 10000) END,
  datetime('2022-01-01 08:00:00', printf('+%d hours', (n * 7) % 8760))
FROM numbers;

WITH RECURSIVE numbers(n) AS (
  SELECT 1
  UNION ALL
  SELECT n + 1 FROM numbers WHERE n < 300
)
INSERT OR IGNORE INTO products
  (id, sku, name, category, unit_price, active, attributes)
SELECT
  n,
  printf('SKU-%05d', n),
  printf('Product %04d', n),
  CASE n % 5 WHEN 0 THEN 'books' WHEN 1 THEN 'home' WHEN 2 THEN 'office' WHEN 3 THEN 'outdoors' ELSE 'electronics' END,
  round(4.99 + ((n * 29) % 200000) / 100.0, 2),
  CASE WHEN n % 7 = 0 THEN 0 ELSE 1 END,
  printf('{"color":"%s","weight_grams":%d}',
    CASE n % 4 WHEN 0 THEN 'blue' WHEN 1 THEN 'green' WHEN 2 THEN 'red' ELSE 'black' END,
    100 + ((n * 17) % 5000))
FROM numbers;

WITH RECURSIVE numbers(n) AS (
  SELECT 1
  UNION ALL
  SELECT n + 1 FROM numbers WHERE n < 3000
)
INSERT OR IGNORE INTO orders
  (id, customer_id, status, ordered_at, total_amount, shipping_address, notes)
SELECT
  n,
  ((n - 1) % 1200) + 1,
  CASE n % 5 WHEN 0 THEN 'pending' WHEN 1 THEN 'paid' WHEN 2 THEN 'shipped' WHEN 3 THEN 'cancelled' ELSE 'refunded' END,
  datetime('2023-01-01 09:00:00', printf('+%d hours', (n * 11) % 17520)),
  round(20.00 + ((n * 113) % 500000) / 100.0, 2),
  printf('%d Example Street, Testville, ZZ %05d', n, n % 10000),
  CASE WHEN n % 6 = 0 THEN NULL ELSE 'Manual test order note ' || n END
FROM numbers;

INSERT OR IGNORE INTO order_items
  (id, order_id, product_id, quantity, unit_price, discount_percent)
SELECT
  ((o.id - 1) * 3) + item_number,
  o.id,
  ((o.id * 7 + item_number * 11) % 300) + 1,
  ((o.id + item_number) % 5) + 1,
  round(4.99 + (((o.id * 7 + item_number * 11) * 29) % 200000) / 100.0, 2),
  CASE item_number WHEN 1 THEN 0.00 WHEN 2 THEN 5.00 ELSE 10.00 END
FROM orders AS o
CROSS JOIN (SELECT 1 AS item_number UNION ALL SELECT 2 UNION ALL SELECT 3);

WITH RECURSIVE numbers(n) AS (
  SELECT 1
  UNION ALL
  SELECT n + 1 FROM numbers WHERE n < 2400
)
INSERT OR IGNORE INTO customer_notes
  (id, customer_id, note_text, metadata, created_at)
SELECT
  n,
  ((n - 1) % 1200) + 1,
  printf('Manual fixture note %d: deliberately verbose text for export and long-cell testing.', n),
  printf('{"source":"seed","priority":"%s"}', CASE n % 3 WHEN 0 THEN 'high' WHEN 1 THEN 'normal' ELSE 'low' END),
  datetime('2023-02-01 10:00:00', printf('+%d hours', (n * 19) % 12000))
FROM numbers;

DROP VIEW IF EXISTS order_summary;
CREATE VIEW order_summary AS
SELECT
  o.id AS order_id,
  o.status,
  o.ordered_at,
  c.id AS customer_id,
  c.name AS customer_name,
  c.email AS customer_email,
  count(oi.id) AS item_count,
  o.total_amount
FROM orders AS o
JOIN customers AS c ON c.id = o.customer_id
LEFT JOIN order_items AS oi ON oi.order_id = o.id
GROUP BY o.id, o.status, o.ordered_at, c.id, c.name, c.email, o.total_amount;

CREATE TABLE IF NOT EXISTS fixture_seed_metadata (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
INSERT OR REPLACE INTO fixture_seed_metadata (key, value)
VALUES
  ('fixture', 'lazysql-manual-v1'),
  ('generated_rows', '1200 customers / 300 products / 3000 orders / 9000 order_items / 2400 notes');

COMMIT;
