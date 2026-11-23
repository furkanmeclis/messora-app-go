-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- Contact fields beyond the minimal Phase 3 record. var1..var5 are free
-- personalisation fields usable in templates; attributes holds any other
-- imported column.
-- squawk-ignore adding-field-with-default
ALTER TABLE contacts
    ADD COLUMN email          citext CHECK (email IS NULL OR length(email) BETWEEN 3 AND 254),
    ADD COLUMN var1           text CHECK (var1 IS NULL OR length(var1) <= 500),
    ADD COLUMN var2           text CHECK (var2 IS NULL OR length(var2) <= 500),
    ADD COLUMN var3           text CHECK (var3 IS NULL OR length(var3) <= 500),
    ADD COLUMN var4           text CHECK (var4 IS NULL OR length(var4) <= 500),
    ADD COLUMN var5           text CHECK (var5 IS NULL OR length(var5) <= 500),
    ADD COLUMN attributes     jsonb NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN opted_out_at   timestamptz,
    ADD COLUMN opt_out_reason text,
    ADD COLUMN source         text NOT NULL DEFAULT 'webhook' CHECK (source IN ('webhook', 'manual', 'import', 'api'));

-- Name search (contains) without a sequential scan.
CREATE EXTENSION IF NOT EXISTS pg_trgm;
-- The table is new in this release line and small; a plain index is fine.
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX contacts_name_trgm_idx ON contacts USING gin (name gin_trgm_ops);
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX contacts_created_idx ON contacts (id DESC);

CREATE TABLE tags (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    name       text NOT NULL CHECK (length(name) BETWEEN 1 AND 60),
    color      text NOT NULL DEFAULT 'gray' CHECK (color ~ '^[a-z]{3,12}$'),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX tags_name_key ON tags (lower(name));

CREATE TABLE contact_tags (
    contact_id uuid NOT NULL REFERENCES contacts (id) ON DELETE CASCADE,
    tag_id     uuid NOT NULL REFERENCES tags (id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (contact_id, tag_id)
);
CREATE INDEX contact_tags_tag_idx ON contact_tags (tag_id, contact_id);

-- Static lists curated by hand (segments are the dynamic counterpart).
CREATE TABLE contact_groups (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    name        text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    description text NOT NULL DEFAULT '' CHECK (length(description) <= 500),
    created_by  uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX contact_groups_name_key ON contact_groups (lower(name));

CREATE TABLE contact_group_members (
    group_id   uuid NOT NULL REFERENCES contact_groups (id) ON DELETE CASCADE,
    contact_id uuid NOT NULL REFERENCES contacts (id) ON DELETE CASCADE,
    added_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, contact_id)
);
CREATE INDEX contact_group_members_contact_idx ON contact_group_members (contact_id);

CREATE TABLE contact_notes (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    contact_id uuid NOT NULL REFERENCES contacts (id) ON DELETE CASCADE,
    author_id  uuid REFERENCES users (id) ON DELETE SET NULL,
    body       text NOT NULL CHECK (length(body) BETWEEN 1 AND 5000),
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX contact_notes_contact_idx ON contact_notes (contact_id, id DESC);

-- Append-only record of opt-in / opt-out decisions (who, when, how).
CREATE TABLE consent_events (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    contact_id uuid NOT NULL REFERENCES contacts (id) ON DELETE CASCADE,
    kind       text NOT NULL CHECK (kind IN ('opt_in', 'opt_out')),
    source     text NOT NULL CHECK (source IN ('manual', 'import', 'api', 'keyword')),
    note       text NOT NULL DEFAULT '' CHECK (length(note) <= 500),
    actor_id   uuid,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX consent_events_contact_idx ON consent_events (contact_id, id DESC);

CREATE TABLE segments (
    id          uuid PRIMARY KEY DEFAULT uuidv7(),
    name        text NOT NULL CHECK (length(name) BETWEEN 1 AND 120),
    description text NOT NULL DEFAULT '' CHECK (length(description) <= 500),
    definition  jsonb NOT NULL,
    created_by  uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX segments_name_key ON segments (lower(name));

-- File imports run by the worker; progress counters are updated in batches.
CREATE TABLE contact_imports (
    id               uuid PRIMARY KEY DEFAULT uuidv7(),
    filename         text NOT NULL CHECK (length(filename) BETWEEN 1 AND 255),
    storage_key      text NOT NULL,
    size_bytes       bigint NOT NULL,
    status           text NOT NULL DEFAULT 'uploaded'
                     CHECK (status IN ('uploaded', 'queued', 'running', 'completed', 'failed', 'cancelled')),
    headers          jsonb NOT NULL DEFAULT '[]'::jsonb,
    preview          jsonb NOT NULL DEFAULT '[]'::jsonb,
    mapping          jsonb NOT NULL DEFAULT '{}'::jsonb,
    duplicate_policy text NOT NULL DEFAULT 'skip' CHECK (duplicate_policy IN ('skip', 'update')),
    add_tags         text[] NOT NULL DEFAULT '{}',
    group_id         uuid REFERENCES contact_groups (id) ON DELETE SET NULL,
    total_rows       bigint NOT NULL DEFAULT 0,
    processed_rows   bigint NOT NULL DEFAULT 0,
    created_count    bigint NOT NULL DEFAULT 0,
    updated_count    bigint NOT NULL DEFAULT 0,
    skipped_count    bigint NOT NULL DEFAULT 0,
    failed_count     bigint NOT NULL DEFAULT 0,
    error_report_key text,
    last_error       text,
    cancel_requested boolean NOT NULL DEFAULT false,
    created_by       uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    started_at       timestamptz,
    finished_at      timestamptz,
    updated_at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX contact_imports_created_idx ON contact_imports (id DESC);

-- +goose Down
-- Down migrations intentionally drop what Up created.
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS contact_imports;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS segments;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS consent_events;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS contact_notes;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS contact_group_members;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS contact_groups;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS contact_tags;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS tags;
DROP INDEX IF EXISTS contacts_created_idx;
DROP INDEX IF EXISTS contacts_name_trgm_idx;
-- squawk-ignore ban-drop-column
ALTER TABLE contacts
    DROP COLUMN IF EXISTS source,
    DROP COLUMN IF EXISTS opt_out_reason,
    DROP COLUMN IF EXISTS opted_out_at,
    DROP COLUMN IF EXISTS attributes,
    DROP COLUMN IF EXISTS var5,
    DROP COLUMN IF EXISTS var4,
    DROP COLUMN IF EXISTS var3,
    DROP COLUMN IF EXISTS var2,
    DROP COLUMN IF EXISTS var1,
    DROP COLUMN IF EXISTS email;
