-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Phase 17 WP1: tenants (docs/PHASE-17.md §5, docs/DATABASE_DESIGN.md
-- "Kiracılar"). Every company is a tenant; its rows carry tenant_id and
-- row-level security limits each connection to the tenant it was opened
-- for (00025–00028).

-- The tenant of the current connection: the pool sets app.tenant_id when a
-- connection is acquired for a request or a job, and resets it on release.
-- Unset (NULL) means "no tenant": no tenant row is visible and no tenant
-- row can be written. A malformed value raises, it never widens access.
-- +goose StatementBegin
CREATE FUNCTION app_tenant_id() RETURNS uuid
LANGUAGE sql STABLE PARALLEL SAFE AS $$
    SELECT nullif(current_setting('app.tenant_id', true), '')::uuid
$$;
-- +goose StatementEnd

-- Sub-domain labels the platform keeps for itself. Keep in sync with
-- tenant.ReservedSlugs (internal/platform/tenant/slug.go); a test compares
-- both lists.
-- +goose StatementBegin
CREATE FUNCTION tenant_reserved_slugs() RETURNS text[]
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT ARRAY['www', 'api', 'admin', 'app', 'hooks', 'webhooks', 'mail', 'smtp', 'imap', 'pop', 'mx',
                 'status', 'docs', 'static', 'assets', 'cdn', 'media', 'files', 'help', 'support', 'blog',
                 'platform', 'console', 'dashboard', 'auth', 'login', 'signup', 'register', 'account',
                 'billing', 'ns1', 'ns2', 'ftp', 'root', 'messora', 'system', 'realtime', 'ws', 'connection',
                 'internal', 'test', 'dev', 'staging', 'demo']::text[]
$$;
-- +goose StatementEnd

CREATE TABLE tenants (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    -- The sub-domain label: lowercase letters, digits and inner hyphens,
    -- 3 to 40 characters.
    slug         text NOT NULL UNIQUE
                 CHECK (slug ~ '^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$' AND slug <> ALL (tenant_reserved_slugs())),
    name         text NOT NULL CHECK (length(name) BETWEEN 1 AND 200),
    status       text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'deleted')),
    suspended_at timestamptz,
    deleted_at   timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

-- The host names a tenant is reached at: its sub-domain of the platform
-- domain and any custom domains. A host belongs to one tenant.
CREATE TABLE tenant_domains (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    tenant_id   uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    hostname    text NOT NULL UNIQUE
                CHECK (hostname = lower(hostname) AND length(hostname) <= 253
                       AND hostname ~ '^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)*$'),
    kind        text NOT NULL CHECK (kind IN ('subdomain', 'custom')),
    -- Custom domains answer only once their DNS is verified (WP2/WP7);
    -- platform sub-domains are verified when created.
    verified_at timestamptz,
    is_primary  boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now()
);
-- The table was created above and is still empty, so a plain index is safe.
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX tenant_domains_tenant_idx ON tenant_domains (tenant_id);
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX tenant_domains_one_primary ON tenant_domains (tenant_id) WHERE is_primary;

-- Platform-wide settings (the platform admin panel, WP2): e-mail, default
-- AI price table, Meta connection methods.
CREATE TABLE platform_settings (
    key        text PRIMARY KEY CHECK (key ~ '^[a-z][a-z0-9_.]{0,99}$'),
    value      jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);

-- Tenant #1 holds every row that existed before tenants. Its slug comes
-- from the workspace's organisation name ("default" when that gives no
-- usable label). Its host names are registered by `messora migrate`
-- (PUBLIC_APP_URL and <slug>.TENANT_BASE_DOMAIN).
INSERT INTO tenants (id, slug, name)
SELECT '00000000-0000-0000-0000-000000000001'::uuid,
       CASE WHEN s ~ '^[a-z0-9][a-z0-9-]{1,38}[a-z0-9]$' AND s <> ALL (tenant_reserved_slugs()) THEN s ELSE 'default' END,
       coalesce(nullif(org, ''), 'Default')
FROM (
    SELECT org, regexp_replace(left(regexp_replace(regexp_replace(
               translate(lower(org), 'çğıöşüâîûéèà', 'cgiosuaiueea'), '[^a-z0-9]+', '-', 'g'), '(^-+|-+$)', '', 'g'), 40),
               '-+$', '') AS s
    FROM (SELECT coalesce((SELECT trim(value ->> 'organization_name') FROM settings WHERE key = 'workspace'), '') AS org) o
) x;

-- Database roles. The owner (the role running migrations) keeps owning
-- every table. messora_app is what the API, worker and scheduler connect
-- as: it is not an owner and cannot bypass row-level security.
-- messora_system bypasses it, for the few cross-tenant paths (the outbox
-- relay and maintenance). `messora migrate` gives both roles their login
-- passwords (POSTGRES_APP_PASSWORD, POSTGRES_SYSTEM_PASSWORD); 00028 grants
-- their privileges. Roles belong to the whole PostgreSQL cluster, so an
-- existing role is reused.
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'messora_app') THEN
        CREATE ROLE messora_app NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;
    END IF;
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'messora_system') THEN
        CREATE ROLE messora_system NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE BYPASSRLS;
    END IF;
    -- A pre-existing role must not have more than it should.
    ALTER ROLE messora_app NOSUPERUSER NOBYPASSRLS;
END;
$$;
-- +goose StatementEnd

-- +goose Down
-- Down migrations intentionally drop what Up created. The roles are left:
-- they belong to the cluster, and 00028 Down already revoked what they had.
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS platform_settings;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS tenant_domains;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS tenants;
-- squawk-ignore ban-drop-function
DROP FUNCTION IF EXISTS tenant_reserved_slugs();
-- squawk-ignore ban-drop-function
DROP FUNCTION IF EXISTS app_tenant_id();
