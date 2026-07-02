CREATE TABLE suggestions (
	id UUID PRIMARY KEY,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
	description_contains TEXT NOT NULL,
	priority INTEGER NOT NULL,
	entry_type TEXT CHECK (entry_type IN ('REVENUE', 'EXPENSE', 'TRANSFER')),
	category_id UUID REFERENCES categories(id) ON DELETE RESTRICT,
	account_id UUID REFERENCES accounts(id) ON DELETE RESTRICT,
	transfer_account_id UUID REFERENCES accounts(id) ON DELETE RESTRICT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_suggestions_user_id ON suggestions(user_id);
