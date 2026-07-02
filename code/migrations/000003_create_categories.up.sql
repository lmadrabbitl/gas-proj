CREATE TABLE categories (
	id UUID PRIMARY KEY,
	parent_id UUID REFERENCES categories(id) ON DELETE RESTRICT,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
	code VARCHAR(50),
	name TEXT NOT NULL,
	type TEXT NOT NULL CHECK (type in ( 'INCOME', 'EXPENSE', 'MOVEMENT' )),
	description TEXT,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	deactivated_at TIMESTAMPTZ
);

CREATE INDEX idx_categories_parent_id ON categories(parent_id);
CREATE INDEX idx_categories_user_id ON categories(user_id);
CREATE UNIQUE INDEX unique_active_category_code
ON categories (user_id, code)
WHERE deactivated_at IS NULL;
