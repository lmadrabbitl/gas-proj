CREATE SEQUENCE transfer_id_seq;

CREATE TABLE transactions (
	id UUID PRIMARY KEY,
	date DATE NOT NULL,
	category_id UUID NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
	description TEXT NOT NULL,
	amount BIGINT NOT NULL,
	account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
	transfer_id BIGINT,
	transfer_account_id UUID REFERENCES accounts(id) ON DELETE RESTRICT,
	is_visible BOOLEAN NOT NULL DEFAULT TRUE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_transactions_user_date ON transactions(user_id, date);
CREATE INDEX idx_transactions_user_category ON transactions(user_id, category_id);
CREATE INDEX idx_transactions_user_transfer ON transactions(user_id, transfer_id) WHERE transfer_id IS NOT NULL;
CREATE INDEX idx_transactions_user_account_date ON transactions(user_id, account_id, date);
CREATE INDEX idx_transactions_user_category_date ON transactions(user_id, category_id, date);



