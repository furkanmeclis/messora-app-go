import { execFileSync } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { expect, test, type BrowserContext, type Page } from '@playwright/test';
import { totp } from './totp';

/**
 * Conversation handoff between the AI and a person (Phase 16C P6), with the
 * fake AI provider (the E2E stack runs AI_FAKE_PROVIDER, see
 * scripts/e2e.sh). Runs after identity.spec.ts; the admin is invited
 * directly.
 *
 * The stack has no Meta mock, so customer messages are seeded in the
 * database, together with the ai:reply task the webhook processor would
 * schedule in the same transaction (see inbox.spec.ts); the worker runs the
 * AI turn for real. The seeded account is disabled, so nothing reaches
 * WhatsApp.
 */

const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';
const RUN = Date.now().toString(36);
const ADMIN = {
    name: 'Hande Handoff',
    email: `hande+${RUN}@example.com`,
    password: 'People-take-over-gladly-47',
};
const PROVIDER = `Fake handoff ${RUN}`;
const PROFILE = `Support ${RUN}`;
const CUSTOMER = 'Selin Devir';

function sql(query: string): string {
    return execFileSync(
        'docker',
        [
            'compose',
            '-p',
            PROJECT,
            'exec',
            '-T',
            'postgres',
            'sh',
            '-c',
            // Rows written here belong to tenant #1, the stack's only tenant
            // (the tenant_id column defaults to the session's app.tenant_id).
            'PGOPTIONS="-c app.tenant_id=00000000-0000-0000-0000-000000000001" psql -v ON_ERROR_STOP=1 -qtA -U "$POSTGRES_USER" -d "$POSTGRES_DB"',
        ],
        { cwd: '..', input: query, encoding: 'utf8' },
    ).trim();
}

test.describe.configure({ mode: 'serial' });

let context: BrowserContext;
let page: Page;
let account = '';
let conversation = '';
const digits = String(Date.now() % 1_000_000).padStart(6, '0');
const display = `+1 555 66${digits.slice(0, 2)} ${digits.slice(2)}`;
const phone = `+90555${digits}7`;

/** A customer message, as the webhook processor would store it. */
function inbound(wamid: string, body: string, schedule: boolean) {
    const id = sql(`
        WITH m AS (INSERT INTO messages (phone_number_id, contact_id, direction, wamid, type, body, status, source)
                   SELECT phone_number_id, contact_id, 'inbound', '${wamid}', 'text', '${body}', 'received', 'webhook'
                   FROM conversations WHERE id = '${conversation}' RETURNING id, occurred_at)
        UPDATE conversations c SET last_message_id = m.id, last_message_at = m.occurred_at,
               last_inbound_at = m.occurred_at, updated_at = now()
        FROM m WHERE c.id = '${conversation}'
        RETURNING m.id;`);
    if (schedule) {
        sql(`INSERT INTO outbox_events (topic, payload) VALUES ('task:ai:reply',
                 jsonb_build_object('conversation_id', '${conversation}', 'message_id', '${id}'));
             INSERT INTO outbox_events (topic, payload) VALUES ('rt:conversation:${conversation}',
                 jsonb_build_object('type', 'message.created', 'v', 1, 'id', gen_random_uuid(), 'ts', now(),
                                    'conversation_id', '${conversation}', 'message_id', '${id}'));
             NOTIFY outbox;`);
    }
    return id;
}

test.beforeAll(async ({ browser }) => {
    const token = `msi_${randomBytes(32).toString('base64url')}`;
    sql(`INSERT INTO user_invitations (email, role_id, token_hash, expires_at)
         SELECT '${ADMIN.email}', id, sha256(convert_to('${token}', 'UTF8')), now() + interval '1 hour'
         FROM roles WHERE key = 'admin';`);
    account =
        sql(`INSERT INTO whatsapp_accounts (name, waba_id, access_token_enc, status)
        VALUES ('Handoff ${RUN}', '7${digits}1', '\\x00', 'disabled') RETURNING id;`);
    conversation = sql(`
        WITH n AS (INSERT INTO phone_numbers (account_id, phone_number_id, display_phone_number, verified_name)
                   VALUES ('${account}', '7${digits}2', '${display}', 'Handoff ${RUN}') RETURNING id),
             c AS (INSERT INTO contacts (phone_e164, profile_name) VALUES ('${phone}', '${CUSTOMER}') RETURNING id),
             m AS (INSERT INTO messages (phone_number_id, contact_id, direction, wamid, type, body, status, source)
                   SELECT n.id, c.id, 'inbound', 'wamid.handoff-${RUN}-1', 'text', 'Merhaba', 'received', 'webhook'
                   FROM n, c RETURNING id, phone_number_id, contact_id, occurred_at)
        INSERT INTO conversations (contact_id, phone_number_id, last_message_id, last_message_at, last_inbound_at, unread_count)
        SELECT contact_id, phone_number_id, id, occurred_at, occurred_at, 1 FROM m
        RETURNING id;`);

    context = await browser.newContext({
        viewport: { width: 1440, height: 900 },
    });
    page = await context.newPage();
    await page.goto(`/invite?token=${token}`);
    await page.getByLabel('Your name').fill(ADMIN.name);
    await page.getByLabel('Password').fill(ADMIN.password);
    await page.getByRole('button', { name: 'Join' }).click();
    await expect(page).toHaveURL(/\/account\/security\?enroll=1/);
    await page.getByRole('button', { name: 'Set up now' }).click();
    const secret = (await page.locator('code.break-all').innerText()).trim();
    await page.getByLabel('Code from the app').fill(totp(secret));
    await page.getByRole('button', { name: 'Turn on' }).click();
    await expect(page.locator('pre span')).toHaveCount(10);
    await page.getByRole('button', { name: 'I saved them' }).click();
});

test.afterAll(async () => {
    sql(`DELETE FROM ai_profiles WHERE name = '${PROFILE}';
         DELETE FROM ai_providers WHERE name = '${PROVIDER}';
         DELETE FROM settings WHERE key = 'ai';`);
    await context?.close();
});

test('set up the fake provider, AI and a profile on the number', async () => {
    await page.goto('/settings/ai/providers');
    await page.getByRole('button', { name: 'Add provider' }).click();
    const dialog = page.getByRole('dialog', { name: 'Connect an AI provider' });
    await dialog.getByLabel('Provider').click();
    await page.getByRole('option', { name: 'Fake (tests)' }).click();
    await dialog.getByLabel('Name').fill(PROVIDER);
    await dialog.getByLabel('API key').fill(`fake-e2e-${RUN}-key`);
    await dialog.getByRole('button', { name: 'Connect' }).click();
    await expect(dialog).toHaveCount(0);
    const row = page
        .getByTestId('ai-provider-row')
        .filter({ hasText: PROVIDER });
    await expect(row.getByTestId('ai-provider-status')).toHaveText('Working');
    await row.getByRole('button', { name: 'Refresh models' }).click();
    await expect(page.getByText('2 models in the catalogue.')).toBeVisible();

    await page.goto('/settings/ai');
    const on = page.getByRole('switch', { name: 'AI is on' });
    if ((await on.getAttribute('aria-checked')) !== 'true') await on.click();
    await page.getByRole('combobox', { name: 'Provider' }).click();
    await page.getByRole('option', { name: PROVIDER, exact: false }).click();
    await page.getByRole('combobox', { name: 'Model' }).click();
    await page.getByRole('option', { name: /fake-small/ }).click();
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    await expect(page.getByText('AI settings saved.')).toBeVisible();

    await page.goto('/ai-profiles/new');
    await page.getByLabel('Profile name').fill(PROFILE);
    await page.getByLabel('WhatsApp number').click();
    await page.getByRole('option', { name: display, exact: false }).click();
    await page
        .getByLabel('Instructions (system prompt)')
        .fill('You are the support assistant of Acme.');
    await page.getByTestId('ai-profile-save').click();
    await expect(page).toHaveURL(/\/ai-profiles\/[0-9a-f-]{36}$/);
});

test('hand the conversation to the AI, see its reply, escalate and take over', async () => {
    await page.goto(`/inbox?c=${conversation}`);
    await expect(page.getByTestId('thread-title')).toHaveText(CUSTOMER);
    const owner = page.getByTestId('owner-badge');
    await expect(owner).toHaveAttribute('data-owner', 'none');
    await page.getByTestId('tab-activity').click();
    await expect(page.getByTestId('activity')).toBeVisible();

    // Hand to the AI: it answers the unanswered "Merhaba" right away.
    await page.getByTestId('hand-to-ai').click();
    await expect(owner).toHaveAttribute('data-owner', 'ai');
    const activity = page.getByTestId('activity-item');
    await expect(
        activity.filter({ hasText: 'handed to the AI assistant' }),
    ).toBeVisible();
    const aiReply = page.locator('[data-testid="message"][data-ai="true"]');
    await expect(aiReply).toContainText('Fake reply to: Merhaba', {
        timeout: 45_000,
    });
    await expect(aiReply.getByTestId('ai-label')).toBeVisible();
    // The timeline follows without a reload.
    await expect(activity.filter({ hasText: 'The AI replied' })).toBeVisible();

    // The customer asks for a person: the AI hands over with a summary.
    inbound(`wamid.handoff-${RUN}-2`, 'Bir temsilci istiyorum lütfen', true);
    const banner = page.getByTestId('escalation-banner');
    await expect(banner).toBeVisible({ timeout: 45_000 });
    await expect(banner).toHaveAttribute('data-reason', 'customer_request');
    await expect(banner).toContainText('The customer asked for a person');
    await expect(
        activity.filter({ hasText: 'handed the conversation to a person' }),
    ).toBeVisible();
    await expect(owner).toHaveAttribute('data-owner', 'human');
    await expect
        .poll(() =>
            sql(
                `SELECT reason || '/' || summary_source FROM conversation_handoffs
                 WHERE conversation_id = '${conversation}' AND from_type = 'ai'`,
            ),
        )
        .toBe('customer_request/ai');

    // The agent takes over: the banner goes, the conversation is theirs.
    await banner.getByTestId('banner-take-over').click();
    await expect(banner).toHaveCount(0);
    await expect(
        activity.filter({ hasText: 'The handover was taken.' }),
    ).toBeVisible();
    await expect
        .poll(() =>
            sql(
                `SELECT owner_type || '/' || (pending_handoff_id IS NULL) FROM conversations WHERE id = '${conversation}'`,
            ),
        )
        .toBe('human/true');
});

test('an internal note never looks like a customer message', async () => {
    await page.goto(`/inbox?c=${conversation}`);
    await page.getByTestId('composer-note').click();
    await page
        .getByTestId('note-composer')
        .getByRole('textbox', { name: 'Note' })
        .fill('Müşteri VIP; kargo ücretsiz.');
    await page.getByTestId('note-for-ai').click();
    await page
        .getByTestId('note-composer')
        .getByRole('button', { name: 'Add note' })
        .click();
    const note = page.getByTestId('internal-note');
    await expect(note).toContainText('Müşteri VIP');
    await expect(note).toHaveAttribute('data-for-ai', 'true');
    await expect(
        page
            .locator('[data-testid="message"]')
            .filter({ hasText: 'Müşteri VIP' }),
    ).toHaveCount(0);
});
