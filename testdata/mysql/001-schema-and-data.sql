-- LazySQL manual fixture for MySQL 8.4.
-- The official image runs this file on the first start of an empty volume.

CREATE DATABASE IF NOT EXISTS lazysql_test
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;
USE lazysql_test;

SET SESSION cte_max_recursion_depth = 10000;
SET FOREIGN_KEY_CHECKS = 0;
DROP VIEW IF EXISTS order_summary;
DROP PROCEDURE IF EXISTS customer_order_total;
DROP TABLE IF EXISTS order_items, customer_notes, orders, products, customers;
SET FOREIGN_KEY_CHECKS = 1;

CREATE TABLE customers (
  id INT NOT NULL,
  external_id VARCHAR(32) NOT NULL,
  name VARCHAR(120) NOT NULL,
  email VARCHAR(160) NOT NULL,
  status VARCHAR(16) NOT NULL,
  signup_date DATE NOT NULL,
  balance DECIMAL(12, 2) NOT NULL,
  preferences JSON NOT NULL,
  phone VARCHAR(32) NULL,
  created_at DATETIME NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_customers_external_id (external_id),
  UNIQUE KEY uq_customers_email (email),
  CONSTRAINT chk_customers_status CHECK (status IN ('active', 'trial', 'paused', 'closed'))
) ENGINE = InnoDB;

CREATE TABLE products (
  id INT NOT NULL,
  sku VARCHAR(32) NOT NULL,
  name VARCHAR(160) NOT NULL,
  category VARCHAR(32) NOT NULL,
  unit_price DECIMAL(10, 2) NOT NULL,
  active BOOLEAN NOT NULL,
  attributes JSON NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_products_sku (sku),
  KEY idx_products_category_active (category, active)
) ENGINE = InnoDB;

CREATE TABLE orders (
  id INT NOT NULL,
  customer_id INT NOT NULL,
  status VARCHAR(16) NOT NULL,
  ordered_at DATETIME NOT NULL,
  total_amount DECIMAL(12, 2) NOT NULL,
  shipping_address TEXT NOT NULL,
  notes TEXT NULL,
  PRIMARY KEY (id),
  KEY idx_orders_customer (customer_id),
  KEY idx_orders_status_ordered_at (status, ordered_at),
  CONSTRAINT fk_orders_customer FOREIGN KEY (customer_id) REFERENCES customers (id),
  CONSTRAINT chk_orders_status CHECK (status IN ('pending', 'paid', 'shipped', 'cancelled', 'refunded'))
) ENGINE = InnoDB;

CREATE TABLE order_items (
  id INT NOT NULL,
  order_id INT NOT NULL,
  product_id INT NOT NULL,
  quantity INT NOT NULL,
  unit_price DECIMAL(10, 2) NOT NULL,
  discount_percent DECIMAL(5, 2) NOT NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_order_items_order_product (order_id, product_id),
  KEY idx_order_items_product (product_id),
  CONSTRAINT fk_order_items_order FOREIGN KEY (order_id) REFERENCES orders (id),
  CONSTRAINT fk_order_items_product FOREIGN KEY (product_id) REFERENCES products (id),
  CONSTRAINT chk_order_items_quantity CHECK (quantity > 0)
) ENGINE = InnoDB;

CREATE TABLE customer_notes (
  id INT NOT NULL,
  customer_id INT NOT NULL,
  note_text TEXT NOT NULL,
  metadata JSON NOT NULL,
  created_at DATETIME NOT NULL,
  PRIMARY KEY (id),
  KEY idx_customer_notes_customer_created (customer_id, created_at),
  CONSTRAINT fk_customer_notes_customer FOREIGN KEY (customer_id) REFERENCES customers (id)
) ENGINE = InnoDB;

INSERT INTO customers
  (id, external_id, name, email, status, signup_date, balance, preferences, phone, created_at)
WITH RECURSIVE numbers AS (
  SELECT 1 AS n
  UNION ALL
  SELECT n + 1 FROM numbers WHERE n < 1200
)
SELECT
  n,
  CONCAT('CUS-', LPAD(n, 6, '0')),
  CONCAT('Customer ', LPAD(n, 4, '0')),
  CONCAT('customer', n, '@example.test'),
  CASE MOD(n, 4)
    WHEN 0 THEN 'active'
    WHEN 1 THEN 'trial'
    WHEN 2 THEN 'paused'
    ELSE 'closed'
  END,
  DATE_ADD('2022-01-01', INTERVAL MOD(n * 13, 730) DAY),
  ROUND(25.50 + MOD(n * 37, 800000) / 100, 2),
  JSON_OBJECT(
    'tier', CASE MOD(n, 3) WHEN 0 THEN 'gold' WHEN 1 THEN 'silver' ELSE 'standard' END,
    'marketing', IF(MOD(n, 3) = 0, TRUE, FALSE)
  ),
  IF(MOD(n, 5) = 0, NULL, CONCAT('+1-555-', LPAD(MOD(n, 10000), 4, '0'))),
  DATE_ADD('2022-01-01 08:00:00', INTERVAL MOD(n * 7, 8760) HOUR)
FROM numbers;

INSERT INTO products
  (id, sku, name, category, unit_price, active, attributes)
WITH RECURSIVE numbers AS (
  SELECT 1 AS n
  UNION ALL
  SELECT n + 1 FROM numbers WHERE n < 300
)
SELECT
  n,
  CONCAT('SKU-', LPAD(n, 5, '0')),
  CONCAT('Product ', LPAD(n, 4, '0')),
  CASE MOD(n, 5)
    WHEN 0 THEN 'books'
    WHEN 1 THEN 'home'
    WHEN 2 THEN 'office'
    WHEN 3 THEN 'outdoors'
    ELSE 'electronics'
  END,
  ROUND(4.99 + MOD(n * 29, 200000) / 100, 2),
  IF(MOD(n, 7) = 0, FALSE, TRUE),
  JSON_OBJECT('color', CASE MOD(n, 4) WHEN 0 THEN 'blue' WHEN 1 THEN 'green' WHEN 2 THEN 'red' ELSE 'black' END,
              'weight_grams', 100 + MOD(n * 17, 5000))
FROM numbers;

INSERT INTO orders
  (id, customer_id, status, ordered_at, total_amount, shipping_address, notes)
WITH RECURSIVE numbers AS (
  SELECT 1 AS n
  UNION ALL
  SELECT n + 1 FROM numbers WHERE n < 3000
)
SELECT
  n,
  MOD(n - 1, 1200) + 1,
  CASE MOD(n, 5)
    WHEN 0 THEN 'pending'
    WHEN 1 THEN 'paid'
    WHEN 2 THEN 'shipped'
    WHEN 3 THEN 'cancelled'
    ELSE 'refunded'
  END,
  DATE_ADD('2023-01-01 09:00:00', INTERVAL MOD(n * 11, 17520) HOUR),
  ROUND(20.00 + MOD(n * 113, 500000) / 100, 2),
  CONCAT(n, ' Example Street, Testville, ZZ ', LPAD(MOD(n, 10000), 5, '0')),
  IF(MOD(n, 6) = 0, NULL, CONCAT('Manual test order note ', n))
FROM numbers;

INSERT INTO order_items
  (id, order_id, product_id, quantity, unit_price, discount_percent)
SELECT
  ((o.id - 1) * 3) + item_number.n,
  o.id,
  MOD(o.id * 7 + item_number.n * 11, 300) + 1,
  MOD(o.id + item_number.n, 5) + 1,
  ROUND(4.99 + MOD((o.id * 7 + item_number.n * 11) * 29, 200000) / 100, 2),
  CASE item_number.n WHEN 1 THEN 0.00 WHEN 2 THEN 5.00 ELSE 10.00 END
FROM orders AS o
CROSS JOIN (SELECT 1 AS n UNION ALL SELECT 2 UNION ALL SELECT 3) AS item_number;

INSERT INTO customer_notes (id, customer_id, note_text, metadata, created_at)
WITH RECURSIVE numbers AS (
  SELECT 1 AS n
  UNION ALL
  SELECT n + 1 FROM numbers WHERE n < 2400
)
SELECT
  n,
  MOD(n - 1, 1200) + 1,
  CONCAT(REPEAT('Manual fixture note ', MOD(n, 4) + 1), '#', n),
  JSON_OBJECT('source', 'seed', 'priority', CASE MOD(n, 3) WHEN 0 THEN 'high' WHEN 1 THEN 'normal' ELSE 'low' END),
  DATE_ADD('2023-02-01 10:00:00', INTERVAL MOD(n * 19, 12000) HOUR)
FROM numbers;

CREATE VIEW order_summary AS
SELECT
  o.id AS order_id,
  o.status,
  o.ordered_at,
  c.id AS customer_id,
  c.name AS customer_name,
  c.email AS customer_email,
  COUNT(oi.id) AS item_count,
  o.total_amount
FROM orders AS o
JOIN customers AS c ON c.id = o.customer_id
LEFT JOIN order_items AS oi ON oi.order_id = o.id
GROUP BY o.id, o.status, o.ordered_at, c.id, c.name, c.email, o.total_amount;

DELIMITER //
CREATE PROCEDURE customer_order_total(IN p_customer_id INT)
BEGIN
  SELECT COALESCE(SUM(total_amount), 0.00) AS total_amount
  FROM orders
  WHERE customer_id = p_customer_id;
END//
DELIMITER ;

CREATE TABLE fixture_seed_metadata (
  `key` VARCHAR(64) NOT NULL PRIMARY KEY,
  `value` VARCHAR(255) NOT NULL
) ENGINE = InnoDB;
INSERT INTO fixture_seed_metadata (`key`, `value`)
VALUES
  ('fixture', 'lazysql-manual-v1'),
  ('generated_rows', '1200 customers / 300 products / 3000 orders / 9000 order_items / 2400 notes');
