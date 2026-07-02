ALTER TABLE investment_assets
ADD COLUMN cnpj VARCHAR(14),
ADD COLUMN metadata_source TEXT,
ADD COLUMN metadata_updated_at TIMESTAMPTZ;
