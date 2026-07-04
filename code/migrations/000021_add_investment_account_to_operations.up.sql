ALTER TABLE investment_operations
ADD COLUMN investment_account_id UUID REFERENCES accounts(id) ON DELETE RESTRICT;

CREATE INDEX idx_investment_operations_user_investment_account_date
ON investment_operations(user_id, investment_account_id, date, created_at, id);
