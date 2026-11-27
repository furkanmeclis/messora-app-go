-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '10min';

-- Phase 17 WP2: global identity and the platform admin panel
-- (docs/PHASE-17.md §5.6, docs/DATABASE_DESIGN.md §1.1, docs/SECURITY.md §3a).
--
-- A person has one global account (users: e-mail unique across the
-- platform, password, two-factor authentication) and a membership with a
-- role in each company (tenant_memberships). Sessions and API keys stay
-- bound to one tenant. Platform admins are global users with
-- is_platform_admin; they sign in on the admin host with their own
-- sessions (platform_sessions).
--
-- Platform work (sign-up, the admin panel) runs as messora_platform: a
-- NOLOGIN, NOBYPASSRLS role that messora_app may switch to (SET ROLE) but
-- does not inherit. It writes the platform tables and sees memberships and
-- sessions of every tenant through explicit policies; every other tenant
-- table stays limited to app.tenant_id as for messora_app.

-- ── Users become global ─────────────────────────────────────────────────────
-- The composite keys to users (tenant_id, id) go first.
-- squawk-ignore ban-drop-constraint
ALTER TABLE user_sessions DROP CONSTRAINT user_sessions_user_id_fkey;
-- squawk-ignore ban-drop-constraint
ALTER TABLE user_roles DROP CONSTRAINT user_roles_user_id_fkey;
-- squawk-ignore ban-drop-constraint
ALTER TABLE users DROP CONSTRAINT users_tenant_id_id_key;
DROP POLICY tenant_isolation ON users;
ALTER TABLE users NO FORCE ROW LEVEL SECURITY;
ALTER TABLE users DISABLE ROW LEVEL SECURITY;

-- Memberships: one per person and company, with the company role. Existing
-- users become members of their tenant with their role and status; the
-- account itself is active (a "disabled" user was disabled in its only
-- company).
CREATE TABLE tenant_memberships (
    tenant_id  uuid NOT NULL DEFAULT app_tenant_id() REFERENCES tenants (id) ON DELETE RESTRICT,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role_id    uuid NOT NULL REFERENCES roles (id) ON DELETE RESTRICT,
    status     text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    invited_by uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, user_id)
);
-- The table was created above and is still empty, so plain indexes are safe.
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX tenant_memberships_user_idx ON tenant_memberships (user_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX tenant_memberships_role_idx ON tenant_memberships (role_id);

-- user_roles is copied and dropped; an owner that is not a superuser would
-- otherwise only see the rows of its (unset) tenant.
ALTER TABLE user_roles NO FORCE ROW LEVEL SECURITY;
ALTER TABLE user_roles DISABLE ROW LEVEL SECURITY;
INSERT INTO tenant_memberships (tenant_id, user_id, role_id, status, created_at, updated_at)
SELECT DISTINCT ON (u.tenant_id, u.id) u.tenant_id, u.id, ur.role_id, u.status, u.created_at, u.updated_at
FROM users u
JOIN user_roles ur ON ur.user_id = u.id
JOIN roles r ON r.id = ur.role_id
ORDER BY u.tenant_id, u.id, r.rank;

-- squawk-ignore ban-drop-table
DROP TABLE user_roles;

-- squawk-ignore ban-drop-column
ALTER TABLE users DROP COLUMN tenant_id;
UPDATE users SET status = 'active' WHERE status <> 'active';
-- Existing accounts were created from an invitation or the setup link, so
-- their address is known to work.
ALTER TABLE users ADD COLUMN email_verified_at timestamptz;
UPDATE users SET email_verified_at = created_at;
ALTER TABLE users ADD COLUMN is_platform_admin boolean NOT NULL DEFAULT false;
-- Set by a platform admin: the next sign-in must go through "forgot password".
ALTER TABLE users ADD COLUMN password_reset_required boolean NOT NULL DEFAULT false;

-- Two-factor recovery codes belong to the person, like the TOTP secret.
DROP POLICY tenant_isolation ON user_recovery_codes;
ALTER TABLE user_recovery_codes NO FORCE ROW LEVEL SECURITY;
ALTER TABLE user_recovery_codes DISABLE ROW LEVEL SECURITY;
-- squawk-ignore ban-drop-column
ALTER TABLE user_recovery_codes DROP COLUMN tenant_id;

-- Sessions stay bound to one tenant. A session of a member requires an
-- active membership (checked on every request); a support session of a
-- platform admin carries the grant it was opened with.
ALTER TABLE user_sessions ADD CONSTRAINT user_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE NOT VALID;
ALTER TABLE user_sessions VALIDATE CONSTRAINT user_sessions_user_id_fkey;
ALTER TABLE user_sessions ADD COLUMN support_grant_id uuid;
ALTER TABLE user_sessions ADD COLUMN read_only boolean NOT NULL DEFAULT false;

-- ── Tenants ─────────────────────────────────────────────────────────────────
-- Subscription state is kept apart from the lifecycle status (active,
-- suspended, deleted): a trial workspace is served like any other. Plans
-- and billing (WP8/WP9) build on these columns.
ALTER TABLE tenants ADD COLUMN subscription_status text NOT NULL DEFAULT 'active';
ALTER TABLE tenants ADD CONSTRAINT tenants_subscription_status_check
    CHECK (subscription_status IN ('trial', 'active', 'past_due', 'canceled')) NOT VALID;
ALTER TABLE tenants VALIDATE CONSTRAINT tenants_subscription_status_check;
ALTER TABLE tenants ADD COLUMN trial_ends_at timestamptz;
ALTER TABLE tenants ADD COLUMN suspended_reason text;
ALTER TABLE tenants ADD CONSTRAINT tenants_suspended_reason_length CHECK (length(suspended_reason) <= 500) NOT VALID;
ALTER TABLE tenants VALIDATE CONSTRAINT tenants_suspended_reason_length;

-- The reserved sub-domains are the ones the sign-up form refuses
-- (frontend/src/lib/auth/slug.ts, tenant.ReservedSlugs; a test compares all
-- three). Existing rows are not re-checked.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION tenant_reserved_slugs() RETURNS text[]
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT ARRAY['www', 'admin', 'api', 'app', 'hooks', 'dokploy', 'cdn', 'mail', 'docs', 'status',
                 'help', 'support']::text[]
$$;
-- +goose StatementEnd

-- Custom domains prove ownership with a TXT record _messora.<domain> whose
-- value is verification_token; until then the domain does not answer.
ALTER TABLE tenant_domains ADD COLUMN verification_token text;
ALTER TABLE tenant_domains ADD COLUMN last_checked_at timestamptz;
ALTER TABLE tenant_domains ADD COLUMN last_check_error text;

-- ── Platform tables ─────────────────────────────────────────────────────────
-- Sessions of the platform admin panel (admin host only, token prefix msp_).
CREATE TABLE platform_sessions (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash   bytea NOT NULL UNIQUE,
    ip           inet,
    user_agent   text,
    reauth_at    timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_seen_at timestamptz NOT NULL DEFAULT now(),
    expires_at   timestamptz NOT NULL,
    revoked_at   timestamptz
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX platform_sessions_user_idx ON platform_sessions (user_id) WHERE revoked_at IS NULL;

-- E-mail verification links of self sign-up. tenant_id names the
-- workspace the person signs in to afterwards.
CREATE TABLE email_verification_tokens (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    tenant_id  uuid NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    used_at    timestamptz
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX email_verification_tokens_user_idx ON email_verification_tokens (user_id);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX email_verification_tokens_tenant_idx ON email_verification_tokens (tenant_id);

-- Support access: a platform admin opens a tenant's panel for a limited
-- time, read-only unless write access is chosen, always with a reason. The
-- company sees its own grants (row-level security below); only the
-- platform role writes them.
CREATE TABLE support_access_grants (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    tenant_id  uuid NOT NULL DEFAULT app_tenant_id() REFERENCES tenants (id) ON DELETE CASCADE,
    admin_id   uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    reason     text NOT NULL CHECK (length(reason) BETWEEN 3 AND 500),
    read_only  boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    ended_at   timestamptz
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX support_access_grants_tenant_idx ON support_access_grants (tenant_id, created_at DESC);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX support_access_grants_admin_idx ON support_access_grants (admin_id);
ALTER TABLE user_sessions ADD CONSTRAINT user_sessions_support_grant_id_fkey
    FOREIGN KEY (support_grant_id) REFERENCES support_access_grants (id) ON DELETE CASCADE NOT VALID;
ALTER TABLE user_sessions VALIDATE CONSTRAINT user_sessions_support_grant_id_fkey;

-- What platform admins did. tenant_id names the company the action
-- touched, if any (the company's own audit log records it as well).
CREATE TABLE platform_audit_logs (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    occurred_at timestamptz NOT NULL DEFAULT now(),
    actor_id    uuid REFERENCES users (id) ON DELETE SET NULL,
    actor_email text,
    action      text NOT NULL CHECK (action ~ '^[a-z_]+\.[a-z_.]+$'),
    entity_type text,
    entity_id   uuid,
    tenant_id   uuid REFERENCES tenants (id) ON DELETE SET NULL,
    request_id  text,
    ip          inet,
    user_agent  text,
    changes     jsonb NOT NULL DEFAULT '{}'
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX platform_audit_logs_occurred_idx ON platform_audit_logs (occurred_at DESC);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX platform_audit_logs_tenant_idx ON platform_audit_logs (tenant_id, occurred_at DESC);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX platform_audit_logs_actor_idx ON platform_audit_logs (actor_id);

-- ── Row-level security of the new tenant table ──────────────────────────────
ALTER TABLE tenant_memberships ENABLE ROW LEVEL SECURITY;
ALTER TABLE tenant_memberships FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON tenant_memberships USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
ALTER TABLE support_access_grants ENABLE ROW LEVEL SECURITY;
ALTER TABLE support_access_grants FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON support_access_grants USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());

-- ── The platform role ───────────────────────────────────────────────────────
-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'messora_platform') THEN
        CREATE ROLE messora_platform NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOBYPASSRLS;
    END IF;
    ALTER ROLE messora_platform NOLOGIN NOSUPERUSER NOBYPASSRLS;
END;
$$;
-- +goose StatementEnd
-- messora_app may switch to it (SET LOCAL ROLE) but does not inherit its
-- privileges.
GRANT messora_platform TO messora_app WITH INHERIT FALSE, SET TRUE;
GRANT USAGE ON SCHEMA public TO messora_platform;

-- The platform registry and catalogue.
GRANT SELECT, INSERT, UPDATE, DELETE ON tenants, tenant_domains, platform_settings,
    meta_rates, meta_rate_markets, ai_model_prices,
    platform_sessions, email_verification_tokens, support_access_grants TO messora_platform;
GRANT SELECT, INSERT ON platform_audit_logs TO messora_platform;
GRANT SELECT ON permissions, roles, role_permissions TO messora_platform;
-- People.
GRANT SELECT, INSERT, UPDATE ON users TO messora_platform;
GRANT SELECT, INSERT, UPDATE, DELETE ON user_recovery_codes TO messora_platform;
-- Across tenants: memberships (who belongs where) and sessions (revoked
-- everywhere when an account is disabled or its password reset).
GRANT SELECT, INSERT, UPDATE ON tenant_memberships TO messora_platform;
GRANT SELECT, INSERT, UPDATE ON user_sessions TO messora_platform;
CREATE POLICY platform_access ON tenant_memberships TO messora_platform USING (true) WITH CHECK (true);
CREATE POLICY platform_access ON user_sessions TO messora_platform USING (true) WITH CHECK (true);
CREATE POLICY platform_access ON support_access_grants TO messora_platform USING (true) WITH CHECK (true);
-- Inside one tenant (app.tenant_id set for the transaction, the usual
-- policies apply): the first settings, the owner invitation and the
-- company audit log entries of platform actions.
GRANT SELECT, INSERT, UPDATE ON settings, user_invitations, audit_logs, password_reset_tokens TO messora_platform;
GRANT SELECT ON api_keys TO messora_platform;

-- messora_app keeps reading the catalogue tables (prices for cost
-- estimates) but no longer writes them: the platform panel does.
REVOKE INSERT, UPDATE, DELETE ON meta_rates, meta_rate_markets, ai_model_prices FROM messora_app;
-- Platform-only tables.
REVOKE ALL ON platform_sessions, email_verification_tokens, platform_audit_logs FROM messora_app;
REVOKE INSERT, UPDATE, DELETE ON support_access_grants FROM messora_app;
-- Global accounts: read and changed by the person (profile, password,
-- two-factor), created on invitation acceptance; never deleted.
REVOKE DELETE ON users FROM messora_app;

-- +goose Down
ALTER TABLE tenant_memberships NO FORCE ROW LEVEL SECURITY;
ALTER TABLE tenant_memberships DISABLE ROW LEVEL SECURITY;
REVOKE ALL ON ALL TABLES IN SCHEMA public FROM messora_platform;
REVOKE USAGE ON SCHEMA public FROM messora_platform;
REVOKE messora_platform FROM messora_app;
GRANT INSERT, UPDATE, DELETE ON meta_rates, meta_rate_markets, ai_model_prices TO messora_app;
GRANT DELETE ON users TO messora_app;
DROP POLICY IF EXISTS platform_access ON user_sessions;
DROP POLICY IF EXISTS platform_access ON support_access_grants;

-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS platform_audit_logs;
-- squawk-ignore ban-drop-column
ALTER TABLE user_sessions DROP COLUMN IF EXISTS support_grant_id;
-- squawk-ignore ban-drop-column
ALTER TABLE user_sessions DROP COLUMN IF EXISTS read_only;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS support_access_grants;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS email_verification_tokens;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS platform_sessions;

-- squawk-ignore ban-drop-column
ALTER TABLE tenant_domains DROP COLUMN IF EXISTS last_check_error;
-- squawk-ignore ban-drop-column
ALTER TABLE tenant_domains DROP COLUMN IF EXISTS last_checked_at;
-- squawk-ignore ban-drop-column
ALTER TABLE tenant_domains DROP COLUMN IF EXISTS verification_token;
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION tenant_reserved_slugs() RETURNS text[]
LANGUAGE sql IMMUTABLE PARALLEL SAFE AS $$
    SELECT ARRAY['www', 'api', 'admin', 'app', 'hooks', 'webhooks', 'mail', 'smtp', 'imap', 'pop', 'mx',
                 'status', 'docs', 'static', 'assets', 'cdn', 'media', 'files', 'help', 'support', 'blog',
                 'platform', 'console', 'dashboard', 'auth', 'login', 'signup', 'register', 'account',
                 'billing', 'ns1', 'ns2', 'ftp', 'root', 'messora', 'system', 'realtime', 'ws', 'connection',
                 'internal', 'test', 'dev', 'staging', 'demo']::text[]
$$;
-- +goose StatementEnd
-- squawk-ignore ban-drop-column
ALTER TABLE tenants DROP COLUMN IF EXISTS suspended_reason;
-- squawk-ignore ban-drop-column
ALTER TABLE tenants DROP COLUMN IF EXISTS trial_ends_at;
-- squawk-ignore ban-drop-column
ALTER TABLE tenants DROP COLUMN IF EXISTS subscription_status;

-- Accounts go back into the tenant of their first membership (a person in
-- several companies cannot be represented any more: Down refuses).
-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM tenant_memberships GROUP BY user_id HAVING count(*) > 1) THEN
        RAISE EXCEPTION 'cannot migrate down: some users belong to more than one tenant';
    END IF;
END;
$$;
-- +goose StatementEnd
ALTER TABLE user_recovery_codes ADD COLUMN tenant_id uuid;
ALTER TABLE users ADD COLUMN tenant_id uuid;
UPDATE users u SET tenant_id = coalesce((SELECT m.tenant_id FROM tenant_memberships m WHERE m.user_id = u.id LIMIT 1),
                                        '00000000-0000-0000-0000-000000000001');
UPDATE users u SET status = 'disabled'
WHERE EXISTS (SELECT 1 FROM tenant_memberships m WHERE m.user_id = u.id AND m.status = 'disabled');
UPDATE user_recovery_codes c SET tenant_id = u.tenant_id FROM users u WHERE u.id = c.user_id;
ALTER TABLE users ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE users ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();
ALTER TABLE users ADD CONSTRAINT users_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;
ALTER TABLE user_recovery_codes ALTER COLUMN tenant_id SET NOT NULL;
ALTER TABLE user_recovery_codes ALTER COLUMN tenant_id SET DEFAULT app_tenant_id();
ALTER TABLE user_recovery_codes ADD CONSTRAINT user_recovery_codes_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenants (id) ON DELETE RESTRICT NOT VALID;
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX users_tenant_id_id_key ON users (tenant_id, id);
-- squawk-ignore disallowed-unique-constraint
ALTER TABLE users ADD CONSTRAINT users_tenant_id_id_key UNIQUE USING INDEX users_tenant_id_id_key;
-- squawk-ignore ban-drop-column
ALTER TABLE users DROP COLUMN IF EXISTS password_reset_required;
-- squawk-ignore ban-drop-column
ALTER TABLE users DROP COLUMN IF EXISTS is_platform_admin;
-- squawk-ignore ban-drop-column
ALTER TABLE users DROP COLUMN IF EXISTS email_verified_at;

CREATE TABLE user_roles (
    user_id   uuid NOT NULL,
    role_id   uuid NOT NULL REFERENCES roles (id) ON DELETE RESTRICT,
    tenant_id uuid NOT NULL DEFAULT app_tenant_id() REFERENCES tenants (id) ON DELETE RESTRICT,
    PRIMARY KEY (user_id, role_id)
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX user_roles_role_idx ON user_roles (role_id);
INSERT INTO user_roles (user_id, role_id, tenant_id) SELECT user_id, role_id, tenant_id FROM tenant_memberships;
ALTER TABLE user_roles ADD CONSTRAINT user_roles_user_id_fkey FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE NOT VALID;
-- squawk-ignore ban-drop-table
DROP TABLE tenant_memberships;

-- squawk-ignore ban-drop-constraint
ALTER TABLE user_sessions DROP CONSTRAINT IF EXISTS user_sessions_user_id_fkey;
ALTER TABLE user_sessions ADD CONSTRAINT user_sessions_user_id_fkey FOREIGN KEY (tenant_id, user_id) REFERENCES users (tenant_id, id) ON DELETE CASCADE NOT VALID;

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON users USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
ALTER TABLE user_recovery_codes ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_recovery_codes FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON user_recovery_codes USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
ALTER TABLE user_roles ENABLE ROW LEVEL SECURITY;
ALTER TABLE user_roles FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_isolation ON user_roles USING (tenant_id = app_tenant_id()) WITH CHECK (tenant_id = app_tenant_id());
GRANT SELECT, INSERT, UPDATE, DELETE ON user_roles TO messora_app, messora_system;
