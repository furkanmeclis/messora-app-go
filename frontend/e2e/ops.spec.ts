import { execFileSync } from 'node:child_process';
import { expect, test, type Page } from '@playwright/test';

/**
 * The operations center on the overview (Phase 16C P9). Needs
 * identity.spec.ts to have run first: the user below reuses the agent's
 * password hash. Like campaigns.spec.ts, the account is seeded disabled, so
 * the real engine fails each send at once and the campaign still completes,
 * its progress arriving over Centrifugo. An escalation is seeded together
 * with the event the AI would publish, and taken over from the overview.
 */

const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';
const USER = { email: 'otto@example.com', password: 'Another-strong-pass-73' };
const PERMISSIONS = [
    'campaigns.read',
    'campaigns.start',
    'campaigns.pause',
    'conversations.read',
    'conversations.read_all',
    'conversations.reply',
    'conversations.assign',
    'automations.read',
    'settings.read',
    'contacts.read',
];
const TAG = String(Date.now()).slice(-6);

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
            'psql -v ON_ERROR_STOP=1 -qtA -U "$POSTGRES_USER" -d "$POSTGRES_DB"',
        ],
        { cwd: '..', input: query, encoding: 'utf8' },
    ).trim();
}

async function signIn(page: Page) {
    await page.goto('/login');
    await page.getByLabel('E-mail').fill(USER.email);
    await page.getByLabel('Password').fill(USER.password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(
        page.getByRole('heading', { name: 'Welcome, Otto' }),
    ).toBeVisible();
}

function watchErrors(page: Page) {
    const errors: string[] = [];
    page.on('pageerror', (e) => errors.push(e.message));
    return () => expect(errors, 'browser errors').toEqual([]);
}

/** An unassigned conversation the AI escalated to the pool. */
function escalation(phone: string, name: string, pn: string): string {
    return sql(`
        WITH c AS (INSERT INTO contacts (phone_e164, name) VALUES ('${phone}', '${name}') RETURNING id),
             m AS (INSERT INTO messages (phone_number_id, contact_id, direction, wamid, type, body, status, source)
                   SELECT '${pn}', id, 'inbound', 'wamid.ops-${TAG}-${phone.slice(-2)}', 'text', 'Bir insanla görüşmek istiyorum',
                          'received', 'webhook' FROM c RETURNING id, contact_id, occurred_at),
             v AS (INSERT INTO conversations (contact_id, phone_number_id, last_message_id, last_message_at, last_inbound_at,
                                              unread_count, owner_type)
                   SELECT contact_id, '${pn}', id, occurred_at, occurred_at, 1, 'human' FROM m RETURNING id),
             h AS (INSERT INTO conversation_handoffs (conversation_id, from_type, to_type, reason, actor_type)
                   SELECT id, 'ai', 'pool', 'customer_request', 'ai' FROM v RETURNING id, conversation_id)
        UPDATE conversations SET pending_handoff_id = h.id FROM h WHERE conversations.id = h.conversation_id
        RETURNING conversations.id;`);
}

test.describe.configure({ mode: 'serial' });

let pn = '';

test.beforeAll(() => {
    const hash = sql(
        `SELECT password_hash FROM users WHERE email = 'ari@example.com'`,
    );
    expect(
        hash,
        'identity.spec.ts must run first (it creates ari@example.com)',
    ).toMatch(/^\$argon2id\$/);
    sql(`
        WITH r AS (INSERT INTO roles (key, name, rank) VALUES ('e2e_ops', 'Operations (E2E)', 25)
                   ON CONFLICT (key) DO UPDATE SET name = EXCLUDED.name RETURNING id),
             p AS (INSERT INTO role_permissions (role_id, permission_key)
                   SELECT r.id, k FROM r, unnest(ARRAY['${PERMISSIONS.join("','")}']) k ON CONFLICT DO NOTHING),
             u AS (INSERT INTO users (email, name, password_hash)
                   SELECT '${USER.email}', 'Otto Operations', password_hash FROM users WHERE email = 'ari@example.com'
                   ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name RETURNING id)
        INSERT INTO user_roles (user_id, role_id) SELECT u.id, r.id FROM u, r ON CONFLICT DO NOTHING;`);
    pn = sql(`
        WITH a AS (INSERT INTO whatsapp_accounts (name, waba_id, access_token_enc, status)
                   VALUES ('E2E Ops ${TAG}', '8001${TAG}', '\\x00', 'disabled') RETURNING id),
             n AS (INSERT INTO phone_numbers (account_id, phone_number_id, display_phone_number, verified_name,
                                              quality_rating, messaging_limit)
                   SELECT id, '8002${TAG}', '+1 555 8${TAG}', 'E2E Ops Shop', 'GREEN', 'TIER_1K' FROM a
                   RETURNING id, account_id),
             t AS (INSERT INTO message_templates (account_id, meta_template_id, name, language, category, status, components)
                   SELECT account_id, 'e2e-ops-${TAG}', 'e2e_ops', 'en_US', 'MARKETING', 'APPROVED',
                          '[{"type":"BODY","text":"Hi, our sale starts today."}]'::jsonb FROM n)
        SELECT id FROM n;`);
    sql(`INSERT INTO contacts (phone_e164, name) VALUES
             ('+90544${TAG}1', 'Ece Ops'), ('+90544${TAG}2', 'Can Ops'), ('+90544${TAG}3', 'Deniz Ops');`);
});

test('the overview shows the sections and follows a campaign live', async ({
    page,
}) => {
    const noErrors = watchErrors(page);
    const campaign = `E2E Ops sale ${TAG}`;
    sql(`
        WITH k AS (INSERT INTO campaigns (name, status, phone_number_id, template_name, template_language,
                                          template_params, audience, total_recipients, started_at, paused_at,
                                          paused_reason)
                   VALUES ('${campaign}', 'paused', '${pn}', 'e2e_ops', 'en_US', '{}', '{"all":true}', 3, now(), now(),
                           'manual') RETURNING id)
        INSERT INTO campaign_recipients (campaign_id, contact_id, phone_e164)
        SELECT k.id, c.id, c.phone_e164 FROM k, contacts c WHERE c.phone_e164 LIKE '+90544${TAG}_';`);
    escalation(`+90533${TAG}1`, `Ayşe Bekliyor ${TAG}`, pn);
    sql(`
        WITH a AS (INSERT INTO automations (name, trigger_type, steps, program)
                   VALUES ('E2E Ops flow ${TAG}', 'message_received', '[]', '[]') RETURNING id)
        INSERT INTO automation_runs (automation_id, automation_version, event_key, event, status, error, error_code,
                                     attempt, finished_at)
        SELECT id, 1, 'ops-${TAG}', '{}', 'failed', 'HTTP step answered 500', 'retries_exhausted', 3, now() FROM a;`);

    await signIn(page);
    await expect(page.getByTestId('ops-center')).toBeVisible();
    await expect(page.getByTestId('realtime-state')).toHaveText('Live');

    const row = page
        .getByTestId('ops-campaign-row')
        .filter({ hasText: campaign });
    await expect(row.getByTestId('campaign-status')).toHaveAttribute(
        'data-status',
        'paused',
    );
    await expect(
        page
            .getByTestId('ops-escalation-row')
            .filter({ hasText: `Ayşe Bekliyor ${TAG}` }),
    ).toContainText('The customer asked for a person');
    const run = page
        .getByTestId('ops-failed-run-row')
        .filter({ hasText: `E2E Ops flow ${TAG}` });
    await expect(run).toContainText('Retries used up');
    await expect(run).toContainText('after 3 retries');
    const completed = Number(
        (await page.getByTestId('kpi-completed').innerText()).split('\n')[0],
    );

    // Resume from the overview with the pre-resume summary; the engine
    // sends, the campaign completes and leaves the list, without a reload.
    await row.getByTestId('resume-open').click();
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByTestId('preview-remaining')).toHaveText('3');
    await dialog.getByTestId('resume-confirm').click();
    await expect(row).toHaveCount(0, { timeout: 60_000 });
    await expect(page.getByTestId('kpi-completed')).toContainText(
        String(completed + 1),
    );

    // System health moved to its own tab.
    await page
        .getByRole('navigation', { name: 'Overview sections' })
        .getByRole('link', { name: 'System' })
        .click();
    await expect(
        page.getByRole('heading', { name: 'System status' }),
    ).toBeVisible();
    noErrors();
});

test('a new escalation appears live and is taken over from the overview', async ({
    page,
}) => {
    const noErrors = watchErrors(page);
    await signIn(page);
    await expect(page.getByTestId('realtime-state')).toHaveText('Live');
    const name = `Burak Canlı ${TAG}`;
    await expect(
        page.getByTestId('ops-escalation-row').filter({ hasText: name }),
    ).toHaveCount(0);

    // The AI escalates: the conversation and the event it publishes.
    const id = escalation(`+90533${TAG}2`, name, pn);
    sql(`INSERT INTO outbox_events (topic, payload) VALUES ('rt:inbox:all',
             jsonb_build_object('type', 'conversation.handoff', 'v', 1, 'id', gen_random_uuid(), 'ts', now(),
                                'conversation_id', '${id}', 'owner_type', 'human', 'ai_mode', 'off',
                                'reason', 'customer_request', 'pending', true, 'assignee_id', null));
         NOTIFY outbox;`);
    const row = page
        .getByTestId('ops-escalation-row')
        .filter({ hasText: name });
    await expect(row).toBeVisible({ timeout: 30_000 });

    await row.getByTestId('ops-take-over').click();
    await expect(row).toHaveCount(0, { timeout: 30_000 });
    expect(
        sql(
            `SELECT pending_handoff_id IS NULL AND assignee_id IS NOT NULL FROM conversations WHERE id = '${id}'`,
        ),
    ).toBe('t');
    noErrors();
});
