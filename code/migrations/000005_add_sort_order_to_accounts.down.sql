ALTER TABLE accounts
DROP CONSTRAINT IF EXISTS accounts_sort_order_positive;

ALTER TABLE accounts
DROP COLUMN IF EXISTS sort_order;
