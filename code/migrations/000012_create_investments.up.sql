CREATE TABLE investment_assets (
	id UUID PRIMARY KEY,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
	code VARCHAR(20) NOT NULL,
	name TEXT NOT NULL,
	asset_type TEXT NOT NULL,
	is_active BOOLEAN NOT NULL DEFAULT TRUE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE (user_id, code)
);

CREATE TABLE investment_operations (
	id UUID PRIMARY KEY,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
	asset_id UUID NOT NULL REFERENCES investment_assets(id) ON DELETE RESTRICT,
	operation_type TEXT NOT NULL,
	date DATE NOT NULL,
	quantity BIGINT NOT NULL,
	unit_price BIGINT NOT NULL,
	fee_amount BIGINT NOT NULL DEFAULT 0,
	gross_amount BIGINT NOT NULL,
	net_amount BIGINT NOT NULL,
	notes TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE investment_positions (
	id UUID PRIMARY KEY,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
	asset_id UUID NOT NULL REFERENCES investment_assets(id) ON DELETE RESTRICT,
	current_quantity BIGINT NOT NULL,
	average_price BIGINT NOT NULL,
	total_cost_basis BIGINT NOT NULL,
	realized_pnl BIGINT NOT NULL,
	last_recalculated_at TIMESTAMPTZ NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE (user_id, asset_id)
);

CREATE TABLE investment_portfolios (
	id UUID PRIMARY KEY,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
	code VARCHAR(50) NOT NULL,
	name TEXT NOT NULL,
	description TEXT NOT NULL DEFAULT '',
	sort_order INTEGER NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE (user_id, code)
);

CREATE TABLE investment_portfolio_assets (
	id UUID PRIMARY KEY,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
	portfolio_id UUID NOT NULL REFERENCES investment_portfolios(id) ON DELETE CASCADE,
	asset_id UUID NOT NULL REFERENCES investment_assets(id) ON DELETE RESTRICT,
	target_allocation_bps INTEGER NOT NULL,
	max_buy_price BIGINT,
	sort_order INTEGER NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE (user_id, portfolio_id, asset_id)
);

CREATE INDEX idx_investment_assets_user_code ON investment_assets(user_id, code);
CREATE INDEX idx_investment_operations_user_asset_date ON investment_operations(user_id, asset_id, date, created_at, id);
CREATE INDEX idx_investment_positions_user_asset ON investment_positions(user_id, asset_id);
CREATE INDEX idx_investment_portfolios_user_sort ON investment_portfolios(user_id, sort_order, created_at);
CREATE INDEX idx_investment_portfolio_assets_user_portfolio_sort ON investment_portfolio_assets(user_id, portfolio_id, sort_order, created_at);
