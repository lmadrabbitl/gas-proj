CREATE TABLE investment_asset_quotes (
	id UUID PRIMARY KEY,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
	asset_code VARCHAR(20) NOT NULL,
	current_price BIGINT NOT NULL,
	quote_updated_at TIMESTAMPTZ NOT NULL,
	source TEXT NOT NULL,
	fetched_at TIMESTAMPTZ NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE (user_id, asset_code)
);

CREATE INDEX idx_investment_asset_quotes_user_code ON investment_asset_quotes(user_id, asset_code);
CREATE INDEX idx_investment_asset_quotes_user_fetched ON investment_asset_quotes(user_id, fetched_at);
