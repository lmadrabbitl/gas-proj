ALTER TABLE accounts
ADD COLUMN asset_role TEXT NOT NULL DEFAULT 'NORMAL';

UPDATE accounts
SET asset_role = CASE
    WHEN type = 'ASSET' AND is_brokerage_account THEN 'BROKERAGE'
    ELSE 'NORMAL'
END;

ALTER TABLE accounts
ADD CONSTRAINT chk_accounts_asset_role
CHECK (
    asset_role IN ('NORMAL', 'BROKERAGE', 'INVESTMENT')
    AND (
        type = 'ASSET'
        OR asset_role = 'NORMAL'
    )
);

ALTER TABLE accounts
DROP COLUMN IF EXISTS is_brokerage_account;
