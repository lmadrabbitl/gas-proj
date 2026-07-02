ALTER TABLE transactions
ADD COLUMN exclude_from_dashboard BOOLEAN NOT NULL DEFAULT FALSE;
