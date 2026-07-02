ALTER TABLE categories
ADD COLUMN sort_order INTEGER;

ALTER TABLE categories
ADD CONSTRAINT categories_sort_order_positive
CHECK (sort_order IS NULL OR sort_order > 0);
