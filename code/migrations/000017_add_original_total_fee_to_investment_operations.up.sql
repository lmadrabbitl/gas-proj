ALTER TABLE investment_operations
ADD COLUMN original_total_fee_amount BIGINT NOT NULL DEFAULT 0;

UPDATE investment_operations
SET original_total_fee_amount = fee_amount
WHERE original_total_fee_amount = 0;
