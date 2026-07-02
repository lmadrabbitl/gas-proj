CREATE TABLE accounts (
	id UUID PRIMARY KEY,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
	code VARCHAR(50),
	name TEXT NOT NULL,
	type TEXT NOT NULL CHECK (type in ( 'ASSET', 'LIABILITY' )),
	balance BIGINT NOT NULL DEFAULT 0,
	currency CHAR(3) NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	deactivated_at TIMESTAMPTZ
);

CREATE INDEX idx_accounts_user ON accounts(user_id);
CREATE UNIQUE INDEX unique_active_account_code
ON accounts (user_id, code)
WHERE deactivated_at IS NULL;
