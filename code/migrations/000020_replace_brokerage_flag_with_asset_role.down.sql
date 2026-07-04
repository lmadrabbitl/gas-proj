ALTER TABLE accounts
ADD COLUMN is_brokerage_account BOOLEAN NOT NULL DEFAULT FALSE;

UPDATE accounts
SET is_brokerage_account = (asset_role = 'BROKERAGE');

ALTER TABLE accounts
DROP CONSTRAINT IF EXISTS chk_accounts_asset_role;

ALTER TABLE accounts
DROP COLUMN IF EXISTS asset_role;
