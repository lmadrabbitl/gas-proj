DROP INDEX IF EXISTS unique_account_code;
DROP INDEX IF EXISTS unique_category_code;

ALTER TABLE accounts
  ALTER COLUMN code DROP NOT NULL;

ALTER TABLE categories
  ALTER COLUMN code DROP NOT NULL;

CREATE UNIQUE INDEX unique_active_account_code
ON accounts (user_id, code)
WHERE deactivated_at IS NULL;

CREATE UNIQUE INDEX unique_active_category_code
ON categories (user_id, code)
WHERE deactivated_at IS NULL;
