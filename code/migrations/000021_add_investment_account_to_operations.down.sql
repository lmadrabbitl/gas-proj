DROP INDEX IF EXISTS idx_investment_operations_user_investment_account_date;

ALTER TABLE investment_operations
DROP COLUMN IF EXISTS investment_account_id;
