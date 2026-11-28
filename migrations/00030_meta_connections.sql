-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Phase 17 WP5: each company connects its own Meta app (docs/PHASE-17.md
-- §3.1, §5.8). Meta's webhooks reach a tenant at its own unguessable
-- address, https://hooks.<base domain>/meta/<webhook key>; the GET
-- handshake checks the tenant's verify token and every POST is signed with
-- one of the tenant's app secrets (whatsapp_accounts.app_secret_enc).

-- The tenant's side of the connection: the webhook key and verify token
-- (sealed, shown in Settings), and the health the panel shows. One row per
-- tenant, created the first time the connection settings are read.
CREATE TABLE meta_connections (
    tenant_id                 uuid PRIMARY KEY DEFAULT app_tenant_id() REFERENCES tenants (id) ON DELETE CASCADE,
    -- own_app (WP5); tech_provider is reserved for Embedded Signup (WP6).
    method                    text NOT NULL DEFAULT 'own_app' CHECK (method IN ('own_app', 'tech_provider')),
    webhook_key_enc           bytea NOT NULL,
    verify_token_enc          bytea NOT NULL,
    webhook_key_rotated_at    timestamptz,
    -- Until then the previous webhook key still answers (rotation grace).
    previous_key_expires_at   timestamptz,
    verify_token_rotated_at   timestamptz,
    last_webhook_at           timestamptz,
    signature_failures        bigint NOT NULL DEFAULT 0,
    last_signature_failure_at timestamptz,
    -- Signed events for a WhatsApp Business Account the tenant has not
    -- connected: dropped, counted and shown.
    waba_mismatches           bigint NOT NULL DEFAULT 0,
    last_waba_mismatch_at     timestamptz,
    last_mismatch_waba_id     text,
    created_at                timestamptz NOT NULL DEFAULT now(),
    updated_at                timestamptz NOT NULL DEFAULT now()
);

ALTER TABLE meta_connections ENABLE ROW LEVEL SECURITY;
ALTER TABLE meta_connections FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON meta_connections USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

-- The routing table of keyed webhook addresses: the SHA-256 of a webhook
-- key names the tenant it belongs to. It is read before any tenant is
-- known (like tenant_domains), so it is a platform table without row-level
-- security, and it holds no secret: a key cannot be recovered from its
-- hash. The current key has no expiry; a rotated-out key keeps answering
-- until expires_at.
CREATE TABLE meta_webhook_keys (
    key_sha256 bytea PRIMARY KEY CHECK (length(key_sha256) = 32),
    tenant_id  uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    expires_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now()
);
-- The table was created above and is still empty, so plain indexes are safe.
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX meta_webhook_keys_tenant_idx ON meta_webhook_keys (tenant_id);
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX meta_webhook_keys_one_current ON meta_webhook_keys (tenant_id) WHERE expires_at IS NULL;

-- The application role only reads the routing table. Its rows are written
-- by this function, which runs as the owner but only ever for the calling
-- connection's tenant (app.tenant_id): a tenant cannot route a key to
-- another tenant, and no request path needs a BYPASSRLS connection.
REVOKE INSERT, UPDATE, DELETE ON meta_webhook_keys FROM messora_app;

-- +goose StatementBegin
CREATE FUNCTION meta_webhook_key_rotate(new_sha256 bytea, grace interval) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public AS $$
DECLARE
    t uuid := app_tenant_id();
BEGIN
    IF t IS NULL THEN
        RAISE EXCEPTION 'meta_webhook_key_rotate needs a tenant' USING ERRCODE = '42501';
    END IF;
    IF new_sha256 IS NULL OR length(new_sha256) <> 32 THEN
        RAISE EXCEPTION 'meta_webhook_key_rotate: a SHA-256 is 32 bytes' USING ERRCODE = '22023';
    END IF;
    IF grace IS NULL OR grace < interval '0' OR grace > interval '30 days' THEN
        RAISE EXCEPTION 'meta_webhook_key_rotate: grace must be between 0 and 30 days' USING ERRCODE = '22023';
    END IF;
    -- One previous key at most: an older one in its grace period ends now.
    DELETE FROM meta_webhook_keys WHERE tenant_id = t AND expires_at IS NOT NULL;
    UPDATE meta_webhook_keys SET expires_at = now() + grace WHERE tenant_id = t AND expires_at IS NULL;
    DELETE FROM meta_webhook_keys WHERE tenant_id = t AND expires_at <= now();
    INSERT INTO meta_webhook_keys (key_sha256, tenant_id) VALUES (new_sha256, t);
END
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION meta_webhook_key_rotate(bytea, interval) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION meta_webhook_key_rotate(bytea, interval) TO messora_app;

-- Platform switch of the second connection method, Messora as a Meta Tech
-- Provider with Embedded Signup (WP6). Off until Meta approves Messora;
-- the platform admin panel turns it on. The other fields are placeholders
-- for WP6 (its app secret will not live here in plain text).
INSERT INTO platform_settings (key, value)
VALUES ('tech_provider', '{"enabled": false, "app_id": null, "configuration_id": null}')
ON CONFLICT (key) DO NOTHING;

-- +goose Down
DELETE FROM platform_settings WHERE key = 'tech_provider';
-- squawk-ignore ban-drop-function
DROP FUNCTION IF EXISTS meta_webhook_key_rotate(bytea, interval);
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS meta_webhook_keys;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS meta_connections;
