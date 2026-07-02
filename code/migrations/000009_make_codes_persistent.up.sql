CREATE OR REPLACE FUNCTION slugify_identifier(value TEXT, fallback_value TEXT)
RETURNS TEXT AS $$
DECLARE
  normalized TEXT;
BEGIN
  normalized := lower(trim(COALESCE(value, '')));
  normalized := translate(
    normalized,
    'àáâãäåāăąćčďèéêëēėęěìíîïīįłľñńòóôõöøōőŕřśšťùúûüūůűýÿžźż',
    'aaaaaaaaacddeeeeeeeiiiiiillnnooooooooorrsstuuuuuuuyyzzz'
  );
  normalized := regexp_replace(normalized, '[^a-z0-9]+', '-', 'g');
  normalized := trim(BOTH '-' FROM normalized);
  normalized := left(normalized, 50);
  normalized := regexp_replace(normalized, '-+$', '', 'g');
  IF normalized = '' THEN
    normalized := fallback_value;
  END IF;
  RETURN normalized;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

DO $$
DECLARE
  rec RECORD;
  base_slug TEXT;
  candidate TEXT;
  suffix INTEGER;
BEGIN
  FOR rec IN
    SELECT id, user_id, name
    FROM accounts
    WHERE code IS NULL OR code = ''
    ORDER BY user_id, created_at, id
  LOOP
    base_slug := slugify_identifier(rec.name, 'account');
    candidate := base_slug;
    suffix := 2;

    WHILE EXISTS (
      SELECT 1
      FROM accounts
      WHERE user_id = rec.user_id
        AND code = candidate
        AND id <> rec.id
    ) LOOP
      candidate := left(base_slug, 50 - char_length('-' || suffix::TEXT)) || '-' || suffix::TEXT;
      candidate := regexp_replace(candidate, '-+$', '', 'g');
      suffix := suffix + 1;
    END LOOP;

    UPDATE accounts
    SET code = candidate
    WHERE id = rec.id;
  END LOOP;
END $$;

DO $$
DECLARE
  rec RECORD;
  base_slug TEXT;
  candidate TEXT;
  suffix INTEGER;
BEGIN
  FOR rec IN
    SELECT id, user_id, name
    FROM categories
    WHERE code IS NULL OR code = ''
    ORDER BY user_id, created_at, id
  LOOP
    base_slug := slugify_identifier(rec.name, 'category');
    candidate := base_slug;
    suffix := 2;

    WHILE EXISTS (
      SELECT 1
      FROM categories
      WHERE user_id = rec.user_id
        AND code = candidate
        AND id <> rec.id
    ) LOOP
      candidate := left(base_slug, 50 - char_length('-' || suffix::TEXT)) || '-' || suffix::TEXT;
      candidate := regexp_replace(candidate, '-+$', '', 'g');
      suffix := suffix + 1;
    END LOOP;

    UPDATE categories
    SET code = candidate
    WHERE id = rec.id;
  END LOOP;
END $$;

ALTER TABLE accounts
  ALTER COLUMN code SET NOT NULL;

ALTER TABLE categories
  ALTER COLUMN code SET NOT NULL;

DROP INDEX IF EXISTS unique_active_account_code;
DROP INDEX IF EXISTS unique_active_category_code;

CREATE UNIQUE INDEX unique_account_code
ON accounts (user_id, code);

CREATE UNIQUE INDEX unique_category_code
ON categories (user_id, code);

DROP FUNCTION slugify_identifier(TEXT, TEXT);
