ALTER TABLE investment_asset_quotes
DROP CONSTRAINT IF EXISTS investment_asset_quotes_user_id_asset_code_key;

DROP INDEX IF EXISTS idx_investment_asset_quotes_user_code;
DROP INDEX IF EXISTS idx_investment_asset_quotes_user_fetched;

DELETE FROM investment_asset_quotes a
USING investment_asset_quotes b
WHERE a.asset_code = b.asset_code
  AND a.fetched_at < b.fetched_at;

ALTER TABLE investment_asset_quotes
DROP COLUMN user_id;

ALTER TABLE investment_asset_quotes
ADD CONSTRAINT investment_asset_quotes_asset_code_key UNIQUE (asset_code);

CREATE INDEX idx_investment_asset_quotes_asset_code ON investment_asset_quotes(asset_code);
CREATE INDEX idx_investment_asset_quotes_fetched ON investment_asset_quotes(fetched_at);
