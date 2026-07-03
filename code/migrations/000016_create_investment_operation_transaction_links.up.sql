CREATE TABLE investment_operation_transaction_links (
	id UUID PRIMARY KEY,
	user_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
	investment_operation_id UUID NOT NULL REFERENCES investment_operations(id) ON DELETE CASCADE,
	transaction_id UUID NOT NULL REFERENCES transactions(id) ON DELETE CASCADE,
	role TEXT NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	UNIQUE (investment_operation_id, transaction_id),
	UNIQUE (investment_operation_id, role)
);

CREATE INDEX idx_inv_op_tx_links_user_op
ON investment_operation_transaction_links(user_id, investment_operation_id);

CREATE INDEX idx_inv_op_tx_links_user_tx
ON investment_operation_transaction_links(user_id, transaction_id);
