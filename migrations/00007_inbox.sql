-- +goose Up
SET LOCAL lock_timeout = '5s';
SET LOCAL statement_timeout = '5min';

-- One conversation per customer and business number: the thread the inbox
-- shows. It is derived from messages and kept current in the same
-- transaction as every message write (internal/modules/messaging).
--
-- last_inbound_at is the last customer message that opens Meta's 24-hour
-- customer service window (reactions and system notices do not); the window
-- of the thread closes at last_inbound_at + 24h. unread_count is shared by
-- the team and reset by "mark read".
CREATE TABLE conversations (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    contact_id      uuid NOT NULL REFERENCES contacts (id) ON DELETE CASCADE,
    phone_number_id uuid NOT NULL REFERENCES phone_numbers (id) ON DELETE CASCADE,
    status          text NOT NULL DEFAULT 'open' CHECK (status IN ('open', 'closed')),
    assignee_id     uuid REFERENCES users (id) ON DELETE SET NULL,
    last_message_id uuid,
    last_message_at timestamptz NOT NULL DEFAULT now(),
    last_inbound_at timestamptz,
    -- squawk-ignore prefer-bigint-over-int
    unread_count    integer NOT NULL DEFAULT 0 CHECK (unread_count >= 0),
    closed_at       timestamptz,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (contact_id, phone_number_id)
);
-- The list is ordered by recency; keyset pagination uses (last_message_at, id).
CREATE INDEX conversations_recent_idx ON conversations (last_message_at DESC, id DESC);
CREATE INDEX conversations_assignee_idx ON conversations (assignee_id, last_message_at DESC, id DESC);
CREATE INDEX conversations_status_idx ON conversations (status, last_message_at DESC, id DESC);
CREATE INDEX conversations_phone_number_idx ON conversations (phone_number_id);

-- History of what people did to a conversation (assignment, closing,
-- reopening). Automatic reopening by a customer message has no actor.
CREATE TABLE conversation_events (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    conversation_id uuid NOT NULL REFERENCES conversations (id) ON DELETE CASCADE,
    kind            text NOT NULL CHECK (kind IN ('assigned', 'unassigned', 'closed', 'reopened')),
    actor_type      text NOT NULL CHECK (actor_type IN ('user', 'api_key', 'system')),
    actor_id        uuid,
    assignee_id     uuid REFERENCES users (id) ON DELETE SET NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX conversation_events_conversation_idx ON conversation_events (conversation_id, id DESC);

-- History queries read one thread (contact and number) newest first.
-- The table may already hold many messages on upgrade.
-- squawk-ignore require-concurrent-index-creation
CREATE INDEX messages_thread_idx ON messages (contact_id, phone_number_id, id DESC);

-- Backfill: one conversation for every thread that already has messages.
INSERT INTO conversations (contact_id, phone_number_id, status, last_message_id, last_message_at, last_inbound_at, created_at)
SELECT m.contact_id, m.phone_number_id, 'open',
       (array_agg(m.id ORDER BY m.occurred_at DESC, m.id DESC))[1],
       max(m.occurred_at),
       max(m.occurred_at) FILTER (WHERE m.direction = 'inbound' AND m.type NOT IN ('reaction', 'unsupported', 'system')),
       min(m.created_at)
FROM messages m
GROUP BY m.contact_id, m.phone_number_id;

-- +goose Down
-- Down migrations intentionally drop what Up created.
DROP INDEX IF EXISTS messages_thread_idx;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS conversation_events;
-- squawk-ignore ban-drop-table
DROP TABLE IF EXISTS conversations;
