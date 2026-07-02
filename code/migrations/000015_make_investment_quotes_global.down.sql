ALTER TABLE investment_asset_quotes
DROP CONSTRAINT IF EXISTS investment_asset_quotes_asset_code_key;

DROP INDEX IF EXISTS idx_investment_asset_quotes_asset_code;
DROP INDEX IF EXISTS idx_investment_asset_quotes_fetched;

ALTER TABLE investment_asset_quotes
ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE RESTRICT;

UPDATE investment_asset_quotes
SET user_id = (SELECT id FROM users ORDER BY id LIMIT 1)
WHERE user_id IS NULL;

ALTER TABLE investment_asset_quotes
ALTER COLUMN user_id SET NOT NULL;

ALTER TABLE investment_asset_quotes
ADD CONSTRAINT investment_asset_quotes_user_id_asset_code_key UNIQUE (user_id, asset_code);

CREATE INDEX idx_investment_asset_quotes_user_code ON investment_asset_quotes(user_id, asset_code);
CREATE INDEX idx_investment_asset_quotes_user_fetched ON investment_asset_quotes(user_id, fetched_at);
