import { execFileSync } from 'node:child_process';
import { expect, test } from '@playwright/test';

/**
 * Inbox journey: an agent opens a conversation, replies and sees the
 * delivery state change live. Runs after identity.spec.ts, which creates the
 * agent.
 *
 * The E2E stack has no Meta mock and no app secret, so signed webhooks
 * cannot be delivered and nothing reaches WhatsApp. The test therefore
 * seeds the customer's message and later the delivery receipt directly in
 * the database, together with the realtime outbox event the webhook
 * processor would write in the same transaction; the worker relays it
 * through Centrifugo like any other event. Webhook processing itself is
 * covered by internal/app/inbox_integration_test.go.
 */

const AGENT = { email: 'ari@example.com', password: 'Another-strong-pass-73' };
const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';

function sql(query: string): string {
    return execFileSync(
        'docker',
        [
            'compose', '-p', PROJECT, 'exec', '-T', 'postgres',
            'sh', '-c', 'psql -v ON_ERROR_STOP=1 -qtA -U "$POSTGRES_USER" -d "$POSTGRES_DB"',
        ],
        { cwd: '..', input: query, encoding: 'utf8' },
    ).trim();
}

/** Announces a conversation change to the inbox, as the API would. */
function announce(conversationId: string) {
    sql(`INSERT INTO outbox_events (topic, payload)
         SELECT 'rt:inbox:all', jsonb_build_object('type', 'conversation.updated', 'v', 1,
                'conversation_id', id, 'status', status, 'unread_count', unread_count)
         FROM conversations WHERE id = '${conversationId}';
         NOTIFY outbox;`);
}

test('an agent opens a conversation, replies and sees the delivery update', async ({ page }) => {
    // A number whose account is disabled (the worker fails the send right
    // away instead of calling Meta) and a customer who just wrote in.
    const conversationId = sql(`
        WITH a AS (INSERT INTO whatsapp_accounts (name, waba_id, access_token_enc, status)
                   VALUES ('E2E', '900000000001', '\\x00', 'disabled') RETURNING id),
             n AS (INSERT INTO phone_numbers (account_id, phone_number_id, display_phone_number, verified_name)
                   SELECT id, '900000000002', '+1 555 0100', 'E2E Shop' FROM a RETURNING id),
             c AS (INSERT INTO contacts (phone_e164, profile_name) VALUES ('+905559990011', 'Deniz Müşteri') RETURNING id),
             m AS (INSERT INTO messages (phone_number_id, contact_id, direction, wamid, type, body, status, source)
                   SELECT n.id, c.id, 'inbound', 'wamid.e2e-1', 'text', 'Siparişim nerede?', 'received', 'webhook'
                   FROM n, c RETURNING id, phone_number_id, contact_id, occurred_at)
        INSERT INTO conversations (contact_id, phone_number_id, last_message_id, last_message_at, last_inbound_at, unread_count)
        SELECT contact_id, phone_number_id, id, occurred_at, occurred_at, 1 FROM m
        RETURNING id;`);
    expect(conversationId).toMatch(/^[0-9a-f-]{36}$/);

    await page.goto('/login');
    await page.getByLabel('E-mail').fill(AGENT.email);
    await page.getByLabel('Password').fill(AGENT.password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(page.getByRole('heading', { name: 'Welcome, Ari' })).toBeVisible();

    // The sidebar leads to the inbox; the list is keyboard reachable.
    await page.getByRole('link', { name: 'Inbox' }).click();
    await expect(page.getByRole('heading', { name: 'Inbox' })).toBeVisible();
    const item = page.getByTestId('conversation-item').filter({ hasText: 'Deniz Müşteri' });
    await expect(item).toContainText('Siparişim nerede?');
    await expect(item.getByTestId('unread-count')).toHaveText('1');
    await item.focus();
    await page.keyboard.press('Enter');

    await expect(page.getByTestId('thread-title')).toHaveText('Deniz Müşteri');
    await expect(page.getByTestId('message').first()).toContainText('Siparişim nerede?');
    await expect(page.getByText(/24h window until/)).toBeVisible();
    // Opening the conversation marks it read for the team.
    await expect.poll(() => sql(`SELECT unread_count FROM conversations WHERE id = '${conversationId}'`)).toBe('0');

    // Reply with the keyboard shortcut.
    const box = page.getByLabel('Reply');
    await box.fill('Kargoya verildi, yarın elinizde.');
    await box.press('Control+Enter');
    const reply = page.locator('[data-testid="message"][data-direction="outbound"]');
    await expect(reply).toContainText('Kargoya verildi');
    await expect(box).toHaveValue('');
    const messageId = sql(`SELECT id FROM messages WHERE direction = 'outbound' AND body LIKE 'Kargoya verildi%'`);
    expect(messageId).toMatch(/^[0-9a-f-]{36}$/);

    // The worker settles the attempt (the seeded account cannot send).
    await expect
        .poll(() => sql(`SELECT status FROM messages WHERE id = '${messageId}'`), { timeout: 30_000 })
        .not.toMatch(/^(pending|sending)$/);

    // Meta's delivery receipt arrives: the bubble updates without a reload.
    sql(`UPDATE messages SET status = 'delivered', status_rank = 4, delivered_at = now(), error_code = NULL,
             error_title = NULL, updated_at = now() WHERE id = '${messageId}';`);
    announce(conversationId);
    await expect(reply).toHaveAttribute('data-status', 'delivered', { timeout: 30_000 });
    await expect(reply.getByRole('img', { name: 'Delivered' })).toBeVisible();

    // Outside the 24-hour window only templates can be sent.
    sql(`UPDATE conversations SET last_inbound_at = now() - interval '25 hours' WHERE id = '${conversationId}';`);
    announce(conversationId);
    await expect(page.getByTestId('window-closed')).toBeVisible({ timeout: 30_000 });
    await expect(page.getByText('24h window closed')).toBeVisible();
});
