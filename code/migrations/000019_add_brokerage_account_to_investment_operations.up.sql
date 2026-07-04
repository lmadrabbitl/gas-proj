ALTER TABLE investment_operations
ADD COLUMN brokerage_account_id UUID REFERENCES accounts(id) ON DELETE RESTRICT;

CREATE INDEX idx_investment_operations_user_brokerage_account_date
ON investment_operations(user_id, brokerage_account_id, date, created_at, id);
