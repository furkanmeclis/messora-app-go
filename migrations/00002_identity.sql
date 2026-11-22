-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

CREATE TABLE users (
    id                uuid PRIMARY KEY DEFAULT uuidv7(),
    email             citext NOT NULL UNIQUE CHECK (length(email) BETWEEN 3 AND 254),
    name              text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    password_hash     text NOT NULL,
    status            text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    totp_secret_enc   bytea,
    totp_confirmed_at timestamptz,
    locale            text NOT NULL DEFAULT 'en' CHECK (locale ~ '^[a-z]{2}(-[A-Z]{2})?$'),
    timezone          text NOT NULL DEFAULT 'UTC' CHECK (length(timezone) BETWEEN 1 AND 64),
    -- squawk-ignore prefer-bigint-over-int
    failed_logins     integer NOT NULL DEFAULT 0, -- consecutive failures, reset on success
    locked_until      timestamptz,
    last_login_at     timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE permissions (
    key         text PRIMARY KEY CHECK (key ~ '^[a-z_]+\.[a-z_]+$'),
    module      text NOT NULL,
    description text NOT NULL
);

CREATE TABLE roles (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    key        text NOT NULL UNIQUE CHECK (key ~ '^[a-z][a-z0-9_]{1,39}$'),
    name       text NOT NULL,
    -- squawk-ignore prefer-bigint-over-smallint
    rank       smallint NOT NULL, -- lower = more privileged; owner 0
    is_system  boolean NOT NULL DEFAULT false,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE role_permissions (
    role_id        uuid NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    permission_key text NOT NULL REFERENCES permissions (key) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_key)
);

-- A user has exactly one role in v1; the table allows more later.
CREATE TABLE user_roles (
    user_id uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role_id uuid NOT NULL REFERENCES roles (id) ON DELETE RESTRICT,
    PRIMARY KEY (user_id, role_id)
);
-- The table was created above and is still empty, so a plain index is safe.
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX user_roles_role_idx ON user_roles (role_id);

CREATE TABLE user_sessions (
    id            uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id       uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash    bytea NOT NULL UNIQUE,
    ip            inet,
    user_agent    text,
    reauth_at     timestamptz, -- last password/TOTP confirmation for sensitive actions
    created_at    timestamptz NOT NULL DEFAULT now(),
    last_seen_at  timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL,
    revoked_at    timestamptz
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX user_sessions_user_idx ON user_sessions (user_id) WHERE revoked_at IS NULL;
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX user_sessions_expiry_idx ON user_sessions (expires_at);

CREATE TABLE user_recovery_codes (
    id        uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id   uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    code_hash bytea NOT NULL,
    used_at   timestamptz,
    UNIQUE (user_id, code_hash)
);

CREATE TABLE user_invitations (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    email       citext NOT NULL,
    role_id     uuid NOT NULL REFERENCES roles (id) ON DELETE CASCADE,
    token_hash  bytea NOT NULL UNIQUE,
    invited_by  uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz NOT NULL,
    accepted_at timestamptz,
    revoked_at  timestamptz
);
-- At most one open invitation per address.
-- squawk-ignore require-concurrent-index-creation
CREATE UNIQUE INDEX user_invitations_open_email_idx ON user_invitations (email)
    WHERE accepted_at IS NULL AND revoked_at IS NULL;

CREATE TABLE password_reset_tokens (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash bytea NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now(),
    expires_at timestamptz NOT NULL,
    used_at    timestamptz
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX password_reset_tokens_user_idx ON password_reset_tokens (user_id);

CREATE TABLE api_keys (
    id           uuid PRIMARY KEY DEFAULT uuidv7(),
    name         text NOT NULL CHECK (length(name) BETWEEN 1 AND 100),
    prefix       text NOT NULL, -- first characters, for display only
    key_hash     bytea NOT NULL UNIQUE,
    scopes       text[] NOT NULL CHECK (cardinality(scopes) > 0),
    created_by   uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at   timestamptz NOT NULL DEFAULT now(),
    last_used_at timestamptz,
    expires_at   timestamptz,
    revoked_at   timestamptz
);

CREATE TABLE idempotency_keys (
    principal       text NOT NULL,
    key             text NOT NULL CHECK (length(key) BETWEEN 8 AND 200),
    request_hash    bytea NOT NULL,
    status          text NOT NULL CHECK (status IN ('in_progress', 'done')),
    -- squawk-ignore prefer-bigint-over-int
    response_status integer,
    response_body   bytea,
    created_at      timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (principal, key)
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX idempotency_keys_created_idx ON idempotency_keys (created_at);

CREATE TABLE audit_logs (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    occurred_at timestamptz NOT NULL DEFAULT now(),
    actor_type  text NOT NULL CHECK (actor_type IN ('user', 'api_key', 'system')),
    actor_id    uuid,
    action      text NOT NULL CHECK (action ~ '^[a-z_]+\.[a-z_.]+$'),
    entity_type text,
    entity_id   uuid,
    request_id  text,
    ip          inet,
    user_agent  text,
    changes     jsonb NOT NULL DEFAULT '{}'::jsonb
);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX audit_logs_entity_idx ON audit_logs (entity_type, entity_id, id DESC);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX audit_logs_actor_idx ON audit_logs (actor_id, id DESC);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX audit_logs_action_idx ON audit_logs (action, id DESC);

-- Audit rows are append-only for the application role.
-- +goose StatementBegin
CREATE FUNCTION audit_logs_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit_logs is append-only';
END;
$$;
-- +goose StatementEnd
CREATE TRIGGER audit_logs_no_update
    BEFORE UPDATE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION audit_logs_immutable();

-- Permissions (module.action).
INSERT INTO permissions (key, module, description) VALUES
    ('contacts.read', 'contacts', 'View contacts, tags and groups'),
    ('contacts.create', 'contacts', 'Create contacts'),
    ('contacts.update', 'contacts', 'Edit contacts, tags, groups and notes'),
    ('contacts.delete', 'contacts', 'Delete contacts'),
    ('contacts.import', 'contacts', 'Import contacts from files'),
    ('contacts.export', 'contacts', 'Export contacts'),
    ('segments.read', 'segments', 'View segments'),
    ('segments.manage', 'segments', 'Create, edit and delete segments'),
    ('campaigns.read', 'campaigns', 'View campaigns and statistics'),
    ('campaigns.create', 'campaigns', 'Create and edit draft campaigns'),
    ('campaigns.start', 'campaigns', 'Start and resume campaigns'),
    ('campaigns.pause', 'campaigns', 'Pause and cancel campaigns'),
    ('campaigns.delete', 'campaigns', 'Delete or archive campaigns'),
    ('conversations.read', 'conversations', 'View conversations assigned to oneself or unassigned'),
    ('conversations.read_all', 'conversations', 'View every conversation'),
    ('conversations.reply', 'conversations', 'Send messages in conversations'),
    ('conversations.assign', 'conversations', 'Assign, close and reopen conversations'),
    ('templates.read', 'templates', 'View message templates'),
    ('templates.manage', 'templates', 'Create, delete and sync templates'),
    ('automations.read', 'automations', 'View automations'),
    ('automations.manage', 'automations', 'Create, edit and toggle automations'),
    ('channels.read', 'channels', 'View WhatsApp accounts and numbers'),
    ('channels.manage', 'channels', 'Connect WhatsApp accounts and change credentials'),
    ('webhooks.manage', 'webhooks', 'Manage outgoing webhooks'),
    ('api_keys.manage', 'api_keys', 'Create and revoke API keys'),
    ('messages.send', 'messages', 'Send messages through the developer API'),
    ('settings.read', 'settings', 'View workspace settings'),
    ('settings.update', 'settings', 'Change workspace settings'),
    ('users.manage', 'users', 'Invite users, change roles and disable accounts'),
    ('audit.read', 'audit', 'View the audit log');

INSERT INTO roles (key, name, rank, is_system) VALUES
    ('owner', 'Owner', 0, true),
    ('admin', 'Admin', 10, true),
    ('manager', 'Manager', 20, true),
    ('agent', 'Agent', 30, true);

-- Owner and Admin: every permission.
INSERT INTO role_permissions (role_id, permission_key)
SELECT r.id, p.key FROM roles r CROSS JOIN permissions p WHERE r.key IN ('owner', 'admin');

INSERT INTO role_permissions (role_id, permission_key)
SELECT r.id, p.key FROM roles r JOIN permissions p ON p.key IN (
    'contacts.read', 'contacts.create', 'contacts.update', 'contacts.delete', 'contacts.import', 'contacts.export',
    'segments.read', 'segments.manage',
    'campaigns.read', 'campaigns.create', 'campaigns.start', 'campaigns.pause', 'campaigns.delete',
    'conversations.read', 'conversations.read_all', 'conversations.reply', 'conversations.assign',
    'templates.read', 'automations.read', 'automations.manage', 'channels.read',
    'settings.read', 'audit.read'
) WHERE r.key = 'manager';

INSERT INTO role_permissions (role_id, permission_key)
SELECT r.id, p.key FROM roles r JOIN permissions p ON p.key IN (
    'contacts.read', 'contacts.update', 'conversations.read', 'conversations.reply', 'templates.read'
) WHERE r.key = 'agent';

-- +goose Down
-- Down migrations intentionally drop what Up created.
DROP TRIGGER IF EXISTS audit_logs_no_update ON audit_logs;
-- squawk-ignore ban-drop-function
DROP FUNCTION IF EXISTS audit_logs_immutable();
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS audit_logs;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS idempotency_keys;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS api_keys;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS password_reset_tokens;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS user_invitations;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS user_recovery_codes;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS user_sessions;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS user_roles;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS role_permissions;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS roles;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS permissions;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS users;
