ALTER TABLE categories
DROP CONSTRAINT IF EXISTS categories_sort_order_positive;

ALTER TABLE categories
DROP COLUMN IF EXISTS sort_order;
