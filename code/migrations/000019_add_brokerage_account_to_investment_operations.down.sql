DROP INDEX IF EXISTS idx_investment_operations_user_brokerage_account_date;

ALTER TABLE investment_operations
DROP COLUMN IF EXISTS brokerage_account_id;
