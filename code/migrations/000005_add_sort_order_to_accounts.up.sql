ALTER TABLE accounts
ADD COLUMN sort_order INTEGER;

ALTER TABLE accounts
ADD CONSTRAINT accounts_sort_order_positive
CHECK (sort_order IS NULL OR sort_order > 0);

WITH ranked_accounts AS (
	SELECT
		id,
		ROW_NUMBER() OVER (
			PARTITION BY user_id
			ORDER BY created_at, id
		) AS sort_order
	FROM accounts
	WHERE deactivated_at IS NULL
)
UPDATE accounts
SET sort_order = ranked_accounts.sort_order
FROM ranked_accounts
WHERE accounts.id = ranked_accounts.id;
