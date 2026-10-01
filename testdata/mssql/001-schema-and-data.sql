-- LazySQL manual fixture for SQL Server 2022.
-- mssql-seed runs this file after the server becomes healthy.

IF DB_ID(N'lazysql_test') IS NULL
BEGIN
  CREATE DATABASE [lazysql_test];
END
GO

USE [lazysql_test];
GO

DROP VIEW IF EXISTS [dbo].[order_summary];
DROP FUNCTION IF EXISTS [dbo].[customer_order_total];
DROP TABLE IF EXISTS [dbo].[order_items];
DROP TABLE IF EXISTS [dbo].[customer_notes];
DROP TABLE IF EXISTS [dbo].[orders];
DROP TABLE IF EXISTS [dbo].[products];
DROP TABLE IF EXISTS [dbo].[customers];

CREATE TABLE [dbo].[customers] (
  [id] int NOT NULL PRIMARY KEY,
  [external_id] varchar(32) NOT NULL UNIQUE,
  [name] varchar(120) NOT NULL,
  [email] varchar(160) NOT NULL UNIQUE,
  [status] varchar(16) NOT NULL CHECK ([status] IN ('active', 'trial', 'paused', 'closed')),
  [signup_date] date NOT NULL,
  [balance] decimal(12, 2) NOT NULL,
  [preferences] nvarchar(max) NOT NULL,
  [phone] varchar(32) NULL,
  [created_at] datetime2 NOT NULL
);

CREATE TABLE [dbo].[products] (
  [id] int NOT NULL PRIMARY KEY,
  [sku] varchar(32) NOT NULL UNIQUE,
  [name] varchar(160) NOT NULL,
  [category] varchar(32) NOT NULL,
  [unit_price] decimal(10, 2) NOT NULL,
  [active] bit NOT NULL,
  [attributes] nvarchar(max) NOT NULL
);

CREATE TABLE [dbo].[orders] (
  [id] int NOT NULL PRIMARY KEY,
  [customer_id] int NOT NULL REFERENCES [dbo].[customers] ([id]),
  [status] varchar(16) NOT NULL CHECK ([status] IN ('pending', 'paid', 'shipped', 'cancelled', 'refunded')),
  [ordered_at] datetime2 NOT NULL,
  [total_amount] decimal(12, 2) NOT NULL,
  [shipping_address] nvarchar(max) NOT NULL,
  [notes] nvarchar(max) NULL
);

CREATE TABLE [dbo].[order_items] (
  [id] int NOT NULL PRIMARY KEY,
  [order_id] int NOT NULL REFERENCES [dbo].[orders] ([id]),
  [product_id] int NOT NULL REFERENCES [dbo].[products] ([id]),
  [quantity] int NOT NULL CHECK ([quantity] > 0),
  [unit_price] decimal(10, 2) NOT NULL,
  [discount_percent] decimal(5, 2) NOT NULL,
  CONSTRAINT [uq_order_items_order_product] UNIQUE ([order_id], [product_id])
);

CREATE TABLE [dbo].[customer_notes] (
  [id] int NOT NULL PRIMARY KEY,
  [customer_id] int NOT NULL REFERENCES [dbo].[customers] ([id]),
  [note_text] nvarchar(max) NOT NULL,
  [metadata] nvarchar(max) NOT NULL,
  [created_at] datetime2 NOT NULL
);

CREATE INDEX [idx_orders_customer] ON [dbo].[orders] ([customer_id]);
CREATE INDEX [idx_orders_status_ordered_at] ON [dbo].[orders] ([status], [ordered_at]);
CREATE INDEX [idx_order_items_product] ON [dbo].[order_items] ([product_id]);
CREATE INDEX [idx_customer_notes_customer_created] ON [dbo].[customer_notes] ([customer_id], [created_at]);
CREATE INDEX [idx_products_category_active] ON [dbo].[products] ([category], [active]);

WITH digits(n) AS (
  SELECT n FROM (VALUES (0), (1), (2), (3), (4), (5), (6), (7), (8), (9)) AS values_table(n)
), numbers(n) AS (
  SELECT TOP (1200)
    ROW_NUMBER() OVER (ORDER BY (SELECT NULL))
  FROM digits AS a
  CROSS JOIN digits AS b
  CROSS JOIN digits AS c
  CROSS JOIN digits AS d
)
INSERT INTO [dbo].[customers]
  ([id], [external_id], [name], [email], [status], [signup_date], [balance], [preferences], [phone], [created_at])
SELECT
  n,
  CONCAT('CUS-', RIGHT(CONCAT('000000', n), 6)),
  CONCAT('Customer ', RIGHT(CONCAT('0000', n), 4)),
  CONCAT('customer', n, '@example.test'),
  CASE n % 4 WHEN 0 THEN 'active' WHEN 1 THEN 'trial' WHEN 2 THEN 'paused' ELSE 'closed' END,
  DATEADD(day, (n * 13) % 730, CONVERT(date, '2022-01-01')),
  CAST(ROUND(25.50 + CAST((n * 37) % 800000 AS decimal(12, 2)) / 100, 2) AS decimal(12, 2)),
  CONCAT('{"tier":"', CASE n % 3 WHEN 0 THEN 'gold' WHEN 1 THEN 'silver' ELSE 'standard' END,
         '","marketing":', CASE WHEN n % 3 = 0 THEN 'true' ELSE 'false' END, '}'),
  CASE WHEN n % 5 = 0 THEN NULL ELSE CONCAT('+1-555-', RIGHT(CONCAT('0000', n % 10000), 4)) END,
  DATEADD(hour, (n * 7) % 8760, CONVERT(datetime2, '2022-01-01T08:00:00'))
FROM numbers;

WITH digits(n) AS (
  SELECT n FROM (VALUES (0), (1), (2), (3), (4), (5), (6), (7), (8), (9)) AS values_table(n)
), numbers(n) AS (
  SELECT TOP (300)
    ROW_NUMBER() OVER (ORDER BY (SELECT NULL))
  FROM digits AS a
  CROSS JOIN digits AS b
  CROSS JOIN digits AS c
)
INSERT INTO [dbo].[products]
  ([id], [sku], [name], [category], [unit_price], [active], [attributes])
SELECT
  n,
  CONCAT('SKU-', RIGHT(CONCAT('00000', n), 5)),
  CONCAT('Product ', RIGHT(CONCAT('0000', n), 4)),
  CASE n % 5 WHEN 0 THEN 'books' WHEN 1 THEN 'home' WHEN 2 THEN 'office' WHEN 3 THEN 'outdoors' ELSE 'electronics' END,
  CAST(ROUND(4.99 + CAST((n * 29) % 200000 AS decimal(10, 2)) / 100, 2) AS decimal(10, 2)),
  CASE WHEN n % 7 = 0 THEN CAST(0 AS bit) ELSE CAST(1 AS bit) END,
  CONCAT('{"color":"', CASE n % 4 WHEN 0 THEN 'blue' WHEN 1 THEN 'green' WHEN 2 THEN 'red' ELSE 'black' END,
         '","weight_grams":', 100 + ((n * 17) % 5000), '}')
FROM numbers;

WITH digits(n) AS (
  SELECT n FROM (VALUES (0), (1), (2), (3), (4), (5), (6), (7), (8), (9)) AS values_table(n)
), numbers(n) AS (
  SELECT TOP (3000)
    ROW_NUMBER() OVER (ORDER BY (SELECT NULL))
  FROM digits AS a
  CROSS JOIN digits AS b
  CROSS JOIN digits AS c
  CROSS JOIN digits AS d
)
INSERT INTO [dbo].[orders]
  ([id], [customer_id], [status], [ordered_at], [total_amount], [shipping_address], [notes])
SELECT
  n,
  ((n - 1) % 1200) + 1,
  CASE n % 5 WHEN 0 THEN 'pending' WHEN 1 THEN 'paid' WHEN 2 THEN 'shipped' WHEN 3 THEN 'cancelled' ELSE 'refunded' END,
  DATEADD(hour, (n * 11) % 17520, CONVERT(datetime2, '2023-01-01T09:00:00')),
  CAST(ROUND(20.00 + CAST((n * 113) % 500000 AS decimal(12, 2)) / 100, 2) AS decimal(12, 2)),
  CONCAT(n, ' Example Street, Testville, ZZ ', RIGHT(CONCAT('00000', n % 10000), 5)),
  CASE WHEN n % 6 = 0 THEN NULL ELSE CONCAT('Manual test order note ', n) END
FROM numbers;

INSERT INTO [dbo].[order_items]
  ([id], [order_id], [product_id], [quantity], [unit_price], [discount_percent])
SELECT
  ((o.[id] - 1) * 3) + item_number.n,
  o.[id],
  ((o.[id] * 7 + item_number.n * 11) % 300) + 1,
  ((o.[id] + item_number.n) % 5) + 1,
  CAST(ROUND(4.99 + CAST(((o.[id] * 7 + item_number.n * 11) * 29) % 200000 AS decimal(10, 2)) / 100, 2) AS decimal(10, 2)),
  CASE item_number.n WHEN 1 THEN 0.00 WHEN 2 THEN 5.00 ELSE 10.00 END
FROM [dbo].[orders] AS o
CROSS JOIN (VALUES (1), (2), (3)) AS item_number(n);

WITH digits(n) AS (
  SELECT n FROM (VALUES (0), (1), (2), (3), (4), (5), (6), (7), (8), (9)) AS values_table(n)
), numbers(n) AS (
  SELECT TOP (2400)
    ROW_NUMBER() OVER (ORDER BY (SELECT NULL))
  FROM digits AS a
  CROSS JOIN digits AS b
  CROSS JOIN digits AS c
  CROSS JOIN digits AS d
)
INSERT INTO [dbo].[customer_notes]
  ([id], [customer_id], [note_text], [metadata], [created_at])
SELECT
  n,
  ((n - 1) % 1200) + 1,
  CONCAT(REPLICATE('Manual fixture note ', (n % 4) + 1), '#', n),
  CONCAT('{"source":"seed","priority":"', CASE n % 3 WHEN 0 THEN 'high' WHEN 1 THEN 'normal' ELSE 'low' END, '"}'),
  DATEADD(hour, (n * 19) % 12000, CONVERT(datetime2, '2023-02-01T10:00:00'))
FROM numbers;
GO

CREATE VIEW [dbo].[order_summary] AS
SELECT
  o.[id] AS [order_id],
  o.[status],
  o.[ordered_at],
  c.[id] AS [customer_id],
  c.[name] AS [customer_name],
  c.[email] AS [customer_email],
  COUNT(oi.[id]) AS [item_count],
  o.[total_amount]
FROM [dbo].[orders] AS o
JOIN [dbo].[customers] AS c ON c.[id] = o.[customer_id]
LEFT JOIN [dbo].[order_items] AS oi ON oi.[order_id] = o.[id]
GROUP BY o.[id], o.[status], o.[ordered_at], c.[id], c.[name], c.[email], o.[total_amount];
GO

CREATE FUNCTION [dbo].[customer_order_total] (@customer_id int)
RETURNS decimal(18, 2)
AS
BEGIN
  DECLARE @total decimal(18, 2);
  SELECT @total = COALESCE(SUM([total_amount]), 0.00)
  FROM [dbo].[orders]
  WHERE [customer_id] = @customer_id;
  RETURN @total;
END;
GO

CREATE TABLE [dbo].[fixture_seed_metadata] (
  [key] varchar(64) NOT NULL PRIMARY KEY,
  [value] varchar(255) NOT NULL
);
INSERT INTO [dbo].[fixture_seed_metadata] ([key], [value])
VALUES
  ('fixture', 'lazysql-manual-v1'),
  ('generated_rows', '1200 customers / 300 products / 3000 orders / 9000 order_items / 2400 notes');
GO
