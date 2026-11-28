-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Phase 17 WP6: Messora as a Meta Tech Provider with Embedded Signup
-- (docs/PHASE-17.md §3.2, §5.9). Off until Meta approves Messora; the
-- platform admin turns it on (platform_settings.tech_provider.enabled).
--
-- A company that connects through Messora's Meta app gets an account with
-- connection_method = 'tech_provider' and the business token of its
-- Embedded Signup. Meta sends the webhooks of every such account to one
-- platform address (https://hooks.<base>/meta/tp), signed with Messora's
-- app secret; meta_routes names the tenant of each WhatsApp Business
-- Account (entry[].id).

-- How the account is connected. A WABA has one account in the whole
-- platform (waba_id is unique across tenants) and so exactly one method:
-- its events are accepted only at the address of that method.
ALTER TABLE whatsapp_accounts
    ADD COLUMN connection_method text NOT NULL DEFAULT 'own_app' CHECK (connection_method IN ('own_app', 'tech_provider'));

-- The routing table of the Tech Provider webhook: WABA → tenant. Read
-- before any tenant is known (like meta_webhook_keys), so it is a platform
-- table without row-level security; it holds no secret.
CREATE TABLE meta_routes (
    waba_id    text PRIMARY KEY CHECK (waba_id ~ '^[0-9]{1,32}$'),
    tenant_id  uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now()
);
-- The table was created above and is still empty, so a plain index is safe.
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX meta_routes_tenant_idx ON meta_routes (tenant_id);

-- The application role only reads the routing table. Its rows are written
-- by the two functions below, which run as the owner but only ever for the
-- calling connection's tenant (app.tenant_id): a tenant can neither route a
-- WABA of another tenant to itself nor remove another tenant's route.
REVOKE INSERT, UPDATE, DELETE ON meta_routes FROM messora_app;

-- meta_route_claim routes waba to the caller's tenant. Claiming one's own
-- route again is a no-op; a WABA routed to another tenant raises MS001.
-- +goose StatementBegin
CREATE FUNCTION meta_route_claim(waba text) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public AS $$
DECLARE
    t uuid := app_tenant_id();
    owner_tenant uuid;
BEGIN
    IF t IS NULL THEN
        RAISE EXCEPTION 'meta_route_claim needs a tenant' USING ERRCODE = '42501';
    END IF;
    IF waba IS NULL OR waba !~ '^[0-9]{1,32}$' THEN
        RAISE EXCEPTION 'meta_route_claim: a WABA id is numeric' USING ERRCODE = '22023';
    END IF;
    INSERT INTO meta_routes (waba_id, tenant_id) VALUES (waba, t) ON CONFLICT (waba_id) DO NOTHING;
    IF FOUND THEN
        RETURN;
    END IF;
    SELECT tenant_id INTO owner_tenant FROM meta_routes WHERE waba_id = waba;
    IF owner_tenant IS DISTINCT FROM t THEN
        RAISE EXCEPTION 'WhatsApp Business Account % is connected to another workspace', waba USING ERRCODE = 'MS001';
    END IF;
END
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION meta_route_claim(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION meta_route_claim(text) TO messora_app;

-- meta_route_release removes the caller's route of waba; true when there
-- was one. Another tenant's route is never touched.
-- +goose StatementBegin
CREATE FUNCTION meta_route_release(waba text) RETURNS boolean
LANGUAGE plpgsql SECURITY DEFINER SET search_path = pg_catalog, public AS $$
DECLARE
    t uuid := app_tenant_id();
BEGIN
    IF t IS NULL THEN
        RAISE EXCEPTION 'meta_route_release needs a tenant' USING ERRCODE = '42501';
    END IF;
    DELETE FROM meta_routes WHERE waba_id = waba AND tenant_id = t;
    RETURN FOUND;
END
$$;
-- +goose StatementEnd
REVOKE ALL ON FUNCTION meta_route_release(text) FROM PUBLIC;
GRANT EXECUTE ON FUNCTION meta_route_release(text) TO messora_app;

-- An Embedded Signup of one WABA, as a resumable state machine:
--   exchange  trade the code for the business token and check what it grants
--   claim     route the WABA to the tenant and store the account (disabled)
--   subscribe subscribe Messora's app to the WABA's webhooks
--   register  register the phone number for Cloud API with its PIN
--   finish    read the numbers, activate the account
-- A failed step keeps its place; a retry starts there. The business token
-- and the PIN are sealed (associated data: the onboarding id).
CREATE TABLE meta_onboardings (
    id                 uuid PRIMARY KEY DEFAULT uuidv7(),
    tenant_id          uuid NOT NULL DEFAULT app_tenant_id() REFERENCES tenants (id) ON DELETE CASCADE,
    waba_id            text NOT NULL CHECK (waba_id ~ '^[0-9]{1,32}$'),
    phone_number_id    text NOT NULL CHECK (phone_number_id ~ '^[0-9]{1,32}$'),
    business_id        text CHECK (business_id ~ '^[0-9]{1,32}$'),
    name               text NOT NULL DEFAULT '' CHECK (length(name) <= 120),
    status             text NOT NULL DEFAULT 'running' CHECK (status IN ('running', 'failed', 'completed', 'cancelled')),
    step               text NOT NULL DEFAULT 'exchange' CHECK (step IN ('exchange', 'claim', 'subscribe', 'register', 'finish', 'done')),
    business_token_enc bytea,
    pin_enc            bytea NOT NULL,
    pin_generated      boolean NOT NULL DEFAULT false,
    account_id         uuid REFERENCES whatsapp_accounts (id) ON DELETE SET NULL,
    attempts           bigint NOT NULL DEFAULT 1,
    error_code         text,
    error_message      text CHECK (length(error_message) <= 1000),
    created_by         uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    completed_at       timestamptz
);
-- One unfinished onboarding per WABA and tenant.
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX meta_onboardings_open ON meta_onboardings (tenant_id, waba_id) WHERE status IN ('running', 'failed');
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX meta_onboardings_tenant_created_idx ON meta_onboardings (tenant_id, created_at DESC);

ALTER TABLE meta_onboardings ENABLE ROW LEVEL SECURITY;
ALTER TABLE meta_onboardings FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON meta_onboardings USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

-- The platform switch keeps its WP5 shape; the platform admin fills the
-- rest (Graph API version, sealed app secret, system user token and verify
-- token). Nothing secret is stored here in plain text.
UPDATE platform_settings
SET value = '{"graph_api_version": null}'::jsonb || value, updated_at = now()
WHERE key = 'tech_provider';

-- +goose Down
UPDATE platform_settings
SET value = jsonb_build_object('enabled', false, 'app_id', value->'app_id', 'configuration_id', value->'configuration_id'), updated_at = now()
WHERE key = 'tech_provider';
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS meta_onboardings;
-- squawk-ignore ban-drop-function
DROP FUNCTION IF EXISTS meta_route_release(text);
-- squawk-ignore ban-drop-function
DROP FUNCTION IF EXISTS meta_route_claim(text);
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS meta_routes;
-- squawk-ignore ban-drop-column
ALTER TABLE whatsapp_accounts DROP COLUMN IF EXISTS connection_method;
