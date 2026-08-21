-- Placeholder photo for every seeded catalog item, so the catalog UI is
-- recognizable during development before real curated photos exist.
-- Temporary: this UPDATE will be superseded by real per-item image_url
-- values once the admin/moderation slice lands.
UPDATE catalog_items SET image_url = 'https://placehold.co/600x400?text=%D0%A4%D0%BE%D1%82%D0%BE';
