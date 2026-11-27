-- +goose Up
-- The AI seed price marker was a tenant setting; the seed is platform data
-- now, written by `messora migrate`, so its marker moves to
-- platform_settings. The tenant setting scopes the reads and writes below
-- for an owner role that row-level security applies to.
SELECT set_config('app.tenant_id', '00000000-0000-0000-0000-000000000001', true);
INSERT INTO platform_settings (key, value)
SELECT key, value FROM settings WHERE key = 'ai_prices_seeded'
ON CONFLICT (key) DO NOTHING;
DELETE FROM settings WHERE key = 'ai_prices_seeded';

-- +goose Down
SELECT set_config('app.tenant_id', '00000000-0000-0000-0000-000000000001', true);
INSERT INTO settings (tenant_id, key, value)
SELECT '00000000-0000-0000-0000-000000000001', key, value FROM platform_settings WHERE key = 'ai_prices_seeded'
ON CONFLICT DO NOTHING;
DELETE FROM platform_settings WHERE key = 'ai_prices_seeded';
