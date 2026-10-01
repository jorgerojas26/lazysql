-- LazySQL manual fixture for PostgreSQL 16.
-- The official image runs this file on the first start of an empty volume.

DROP VIEW IF EXISTS public.order_summary;
DROP FUNCTION IF EXISTS public.customer_order_total(integer);
DROP TABLE IF EXISTS public.order_items, public.customer_notes, public.orders, public.products, public.customers CASCADE;

CREATE TABLE public.customers (
  id integer PRIMARY KEY,
  external_id varchar(32) NOT NULL UNIQUE,
  name varchar(120) NOT NULL,
  email varchar(160) NOT NULL UNIQUE,
  status varchar(16) NOT NULL CHECK (status IN ('active', 'trial', 'paused', 'closed')),
  signup_date date NOT NULL,
  balance numeric(12, 2) NOT NULL,
  preferences jsonb NOT NULL,
  phone varchar(32),
  created_at timestamp NOT NULL
);

CREATE TABLE public.products (
  id integer PRIMARY KEY,
  sku varchar(32) NOT NULL UNIQUE,
  name varchar(160) NOT NULL,
  category varchar(32) NOT NULL,
  unit_price numeric(10, 2) NOT NULL,
  active boolean NOT NULL,
  attributes jsonb NOT NULL
);

CREATE TABLE public.orders (
  id integer PRIMARY KEY,
  customer_id integer NOT NULL REFERENCES public.customers (id),
  status varchar(16) NOT NULL CHECK (status IN ('pending', 'paid', 'shipped', 'cancelled', 'refunded')),
  ordered_at timestamp NOT NULL,
  total_amount numeric(12, 2) NOT NULL,
  shipping_address text NOT NULL,
  notes text
);

CREATE TABLE public.order_items (
  id integer PRIMARY KEY,
  order_id integer NOT NULL REFERENCES public.orders (id),
  product_id integer NOT NULL REFERENCES public.products (id),
  quantity integer NOT NULL CHECK (quantity > 0),
  unit_price numeric(10, 2) NOT NULL,
  discount_percent numeric(5, 2) NOT NULL,
  UNIQUE (order_id, product_id)
);

CREATE TABLE public.customer_notes (
  id integer PRIMARY KEY,
  customer_id integer NOT NULL REFERENCES public.customers (id),
  note_text text NOT NULL,
  metadata jsonb NOT NULL,
  created_at timestamp NOT NULL
);

CREATE INDEX idx_orders_customer ON public.orders (customer_id);
CREATE INDEX idx_orders_status_ordered_at ON public.orders (status, ordered_at);
CREATE INDEX idx_order_items_product ON public.order_items (product_id);
CREATE INDEX idx_customer_notes_customer_created ON public.customer_notes (customer_id, created_at);
CREATE INDEX idx_products_category_active ON public.products (category, active);

INSERT INTO public.customers
  (id, external_id, name, email, status, signup_date, balance, preferences, phone, created_at)
SELECT
  n,
  'CUS-' || lpad(n::text, 6, '0'),
  'Customer ' || lpad(n::text, 4, '0'),
  'customer' || n || '@example.test',
  CASE n % 4 WHEN 0 THEN 'active' WHEN 1 THEN 'trial' WHEN 2 THEN 'paused' ELSE 'closed' END,
  date '2022-01-01' + ((n * 13) % 730),
  round((25.50 + ((n * 37) % 800000) / 100.0)::numeric, 2),
  jsonb_build_object(
    'tier', CASE n % 3 WHEN 0 THEN 'gold' WHEN 1 THEN 'silver' ELSE 'standard' END,
    'marketing', (n % 3 = 0)
  ),
  CASE WHEN n % 5 = 0 THEN NULL ELSE '+1-555-' || lpad((n % 10000)::text, 4, '0') END,
  timestamp '2022-01-01 08:00:00' + ((n * 7) % 8760) * interval '1 hour'
FROM generate_series(1, 1200) AS series(n);

INSERT INTO public.products
  (id, sku, name, category, unit_price, active, attributes)
SELECT
  n,
  'SKU-' || lpad(n::text, 5, '0'),
  'Product ' || lpad(n::text, 4, '0'),
  CASE n % 5 WHEN 0 THEN 'books' WHEN 1 THEN 'home' WHEN 2 THEN 'office' WHEN 3 THEN 'outdoors' ELSE 'electronics' END,
  round((4.99 + ((n * 29) % 200000) / 100.0)::numeric, 2),
  (n % 7 <> 0),
  jsonb_build_object(
    'color', CASE n % 4 WHEN 0 THEN 'blue' WHEN 1 THEN 'green' WHEN 2 THEN 'red' ELSE 'black' END,
    'weight_grams', 100 + ((n * 17) % 5000)
  )
FROM generate_series(1, 300) AS series(n);

INSERT INTO public.orders
  (id, customer_id, status, ordered_at, total_amount, shipping_address, notes)
SELECT
  n,
  ((n - 1) % 1200) + 1,
  CASE n % 5 WHEN 0 THEN 'pending' WHEN 1 THEN 'paid' WHEN 2 THEN 'shipped' WHEN 3 THEN 'cancelled' ELSE 'refunded' END,
  timestamp '2023-01-01 09:00:00' + ((n * 11) % 17520) * interval '1 hour',
  round((20.00 + ((n * 113) % 500000) / 100.0)::numeric, 2),
  n || ' Example Street, Testville, ZZ ' || lpad((n % 10000)::text, 5, '0'),
  CASE WHEN n % 6 = 0 THEN NULL ELSE 'Manual test order note ' || n END
FROM generate_series(1, 3000) AS series(n);

INSERT INTO public.order_items
  (id, order_id, product_id, quantity, unit_price, discount_percent)
SELECT
  ((o.id - 1) * 3) + item_number,
  o.id,
  ((o.id * 7 + item_number * 11) % 300) + 1,
  ((o.id + item_number) % 5) + 1,
  round((4.99 + (((o.id * 7 + item_number * 11) * 29) % 200000) / 100.0)::numeric, 2),
  CASE item_number WHEN 1 THEN 0.00 WHEN 2 THEN 5.00 ELSE 10.00 END
FROM public.orders AS o
CROSS JOIN generate_series(1, 3) AS items(item_number);

INSERT INTO public.customer_notes (id, customer_id, note_text, metadata, created_at)
SELECT
  n,
  ((n - 1) % 1200) + 1,
  repeat('Manual fixture note ', (n % 4) + 1) || '#' || n,
  jsonb_build_object(
    'source', 'seed',
    'priority', CASE n % 3 WHEN 0 THEN 'high' WHEN 1 THEN 'normal' ELSE 'low' END
  ),
  timestamp '2023-02-01 10:00:00' + ((n * 19) % 12000) * interval '1 hour'
FROM generate_series(1, 2400) AS series(n);

CREATE VIEW public.order_summary AS
SELECT
  o.id AS order_id,
  o.status,
  o.ordered_at,
  c.id AS customer_id,
  c.name AS customer_name,
  c.email AS customer_email,
  count(oi.id)::integer AS item_count,
  o.total_amount
FROM public.orders AS o
JOIN public.customers AS c ON c.id = o.customer_id
LEFT JOIN public.order_items AS oi ON oi.order_id = o.id
GROUP BY o.id, o.status, o.ordered_at, c.id, c.name, c.email, o.total_amount;

CREATE OR REPLACE FUNCTION public.customer_order_total(p_customer_id integer)
RETURNS numeric
LANGUAGE sql
STABLE
AS $$
  SELECT coalesce(sum(total_amount), 0.00)
  FROM public.orders
  WHERE customer_id = p_customer_id;
$$;

CREATE TABLE public.fixture_seed_metadata (
  key text PRIMARY KEY,
  value text NOT NULL
);
INSERT INTO public.fixture_seed_metadata (key, value)
VALUES ('fixture', 'lazysql-manual-v1'), ('generated_rows', '1200 customers / 300 products / 3000 orders / 9000 order_items / 2400 notes');
