import { execFileSync } from 'node:child_process';
import { expect, test, type Page } from '@playwright/test';

/**
 * Channels, templates and campaigns in the panel. Needs identity.spec.ts to
 * have run first: the marketing user below reuses the agent's password hash
 * (the API only stores argon2id hashes, which the test cannot compute).
 *
 * The E2E stack has no Meta mock, so connecting an account reaches the real
 * Graph API with a made-up token and must be refused. Everything that needs
 * a connected account (numbers, an approved template, a campaign) is seeded
 * in the database instead, like inbox.spec.ts does. The seeded account is
 * disabled, so the worker fails each send right away instead of calling
 * Meta, and the campaign still runs to completion through the real engine,
 * with its progress arriving over Centrifugo.
 */

const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';
const USER = { email: 'mona@example.com', password: 'Another-strong-pass-73' };
const PERMISSIONS = [
    'channels.read', 'channels.manage', 'templates.read', 'templates.manage',
    'campaigns.read', 'campaigns.create', 'campaigns.start', 'campaigns.pause', 'campaigns.delete',
    'segments.read', 'contacts.read',
];
// Unique per run so the spec can be repeated on a kept stack.
const TAG = String(Date.now()).slice(-6);

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

const count = (query: string) => Number(sql(query));

async function signIn(page: Page) {
    await page.goto('/login');
    await page.getByLabel('E-mail').fill(USER.email);
    await page.getByLabel('Password').fill(USER.password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(page.getByRole('heading', { name: 'Welcome, Mona' })).toBeVisible();
}

function watchErrors(page: Page) {
    const errors: string[] = [];
    page.on('pageerror', (e) => errors.push(e.message));
    return () => expect(errors, 'browser errors').toEqual([]);
}

test.describe.configure({ mode: 'serial' });

test.beforeAll(() => {
    const hash = sql(`SELECT password_hash FROM users WHERE email = 'ari@example.com'`);
    expect(hash, 'identity.spec.ts must run first (it creates ari@example.com)').toMatch(/^\$argon2id\$/);
    // A role with exactly what these screens need, so the test does not
    // depend on the owner's second factor.
    sql(`
        WITH r AS (INSERT INTO roles (key, name, rank) VALUES ('e2e_marketing', 'Marketing (E2E)', 25)
                   ON CONFLICT (key) DO UPDATE SET name = EXCLUDED.name RETURNING id),
             p AS (INSERT INTO role_permissions (role_id, permission_key)
                   SELECT r.id, k FROM r, unnest(ARRAY['${PERMISSIONS.join("','")}']) k ON CONFLICT DO NOTHING),
             u AS (INSERT INTO users (email, name, password_hash)
                   SELECT '${USER.email}', 'Mona Marketing', password_hash FROM users WHERE email = 'ari@example.com'
                   ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name RETURNING id)
        INSERT INTO user_roles (user_id, role_id) SELECT u.id, r.id FROM u, r ON CONFLICT DO NOTHING;`);
});

test('channels: the connect form validates input and shows why Meta refused', async ({ page }) => {
    const noErrors = watchErrors(page);
    await signIn(page);
    await page.getByRole('link', { name: 'Channels' }).click();
    await expect(page.getByRole('heading', { name: 'Channels', level: 1 })).toBeVisible();
    if (count('SELECT count(*) FROM whatsapp_accounts') === 0) {
        await expect(page.getByText('No WhatsApp account connected')).toBeVisible();
    }

    // Webhook values for the Meta App dashboard, ready to copy.
    await expect(page.getByRole('textbox', { name: 'Callback URL' })).toHaveValue(/\/webhooks\/whatsapp$/);
    await expect(page.getByRole('textbox', { name: 'Verify token' })).not.toHaveValue('');
    await expect(page.getByRole('button', { name: 'Copy verify token' })).toBeVisible();

    await page.getByRole('button', { name: 'Connect account' }).click();
    const dialog = page.getByRole('dialog');
    const name = dialog.getByLabel('Name', { exact: true });
    const waba = dialog.getByLabel('WhatsApp Business Account ID');
    const submit = dialog.getByRole('button', { name: 'Connect', exact: true });

    // The browser checks required fields and formats before anything is sent.
    await submit.click();
    await expect.poll(() => name.evaluate((el) => (el as HTMLInputElement).validity.valueMissing)).toBe(true);
    await name.fill('E2E Brand');
    await waba.fill('not-a-number');
    await dialog.getByLabel('Access token').fill('EAAG' + 'x'.repeat(40));
    await submit.click();
    await expect.poll(() => waba.evaluate((el) => (el as HTMLInputElement).validity.patternMismatch)).toBe(true);

    // A well-formed but made-up token: Meta (or the network) refuses it and
    // the API's message is shown in the dialog; nothing is stored.
    await waba.fill(`9009${TAG}`);
    await submit.click();
    await expect(dialog.getByRole('alert').filter({ hasText: /Meta/ })).toBeVisible({ timeout: 75_000 });
    await expect(name).toHaveValue('E2E Brand');
    expect(count(`SELECT count(*) FROM whatsapp_accounts WHERE waba_id = '9009${TAG}'`)).toBe(0);
    noErrors();
});

test('templates and campaigns start empty; the wizard needs a phone number', async ({ page }) => {
    const noErrors = watchErrors(page);
    await signIn(page);

    await page.getByRole('link', { name: 'Templates' }).click();
    await expect(page.getByRole('heading', { name: 'Templates', level: 1 })).toBeVisible();
    if (count('SELECT count(*) FROM message_templates') === 0) {
        await expect(page.getByText('No templates yet')).toBeVisible();
    }
    await page.getByRole('combobox', { name: 'Status' }).click();
    await page.getByRole('option', { name: 'Rejected' }).click();
    await expect(page).toHaveURL(/status=REJECTED/);
    if (count(`SELECT count(*) FROM message_templates WHERE status = 'REJECTED'`) === 0) {
        await expect(page.getByText('No templates match these filters')).toBeVisible();
    }

    await page.getByRole('link', { name: 'Campaigns' }).click();
    await expect(page.getByRole('heading', { name: 'Campaigns', level: 1 })).toBeVisible();
    if (count('SELECT count(*) FROM campaigns') === 0) {
        await expect(page.getByText('No campaigns yet')).toBeVisible();
    }

    await page.getByRole('link', { name: 'New campaign' }).click();
    await expect(page.getByRole('heading', { name: '1. Basics' })).toBeVisible();
    await page.getByRole('button', { name: 'Next' }).click();
    await expect(page.getByText('Give the campaign a name.')).toBeVisible();
    await page.getByLabel('Name', { exact: true }).fill('Without a number');
    if (count('SELECT count(*) FROM phone_numbers') === 0) {
        await expect(page.getByTestId('no-phone-number')).toContainText('No phone number connected');
        await page.getByRole('button', { name: 'Next' }).click();
        // The wizard stays on the first step.
        await expect(page.getByRole('heading', { name: '1. Basics' })).toBeVisible();
    } else {
        await expect(page.getByRole('combobox', { name: 'Sender' })).toBeVisible();
    }
    noErrors();
});

/** A disabled account with a number, an approved template and contacts. */
function seedChannel() {
    sql(`
        WITH a AS (INSERT INTO whatsapp_accounts (name, waba_id, access_token_enc, status)
                   VALUES ('E2E Campaigns ${TAG}', '9001${TAG}', '\\x00', 'disabled') RETURNING id),
             n AS (INSERT INTO phone_numbers (account_id, phone_number_id, display_phone_number, verified_name,
                                              quality_rating, messaging_limit)
                   SELECT id, '9002${TAG}', '+1 555 ${TAG}', 'E2E Campaign Shop', 'GREEN', 'TIER_1K' FROM a
                   RETURNING account_id)
        INSERT INTO message_templates (account_id, meta_template_id, name, language, category, status, components)
        SELECT account_id, 'e2e-${TAG}', 'e2e_promo', 'en_US', 'MARKETING', 'APPROVED',
               '[{"type":"BODY","text":"Hi {{1}}, our sale starts today."},{"type":"FOOTER","text":"Reply STOP to opt out"}]'::jsonb
        FROM n;
        INSERT INTO contacts (phone_e164, name) VALUES
            ('+90555${TAG}1', 'Ece Alıcı'), ('+90555${TAG}2', 'Can Alıcı'), ('+90555${TAG}3', '');`);
}

test('a paused campaign resumes and its dashboard follows it live', async ({ page }) => {
    const noErrors = watchErrors(page);
    seedChannel();
    // Paused after recipients were collected: three waiting, one of them
    // without a name for {{first_name}} (and no fallback).
    const campaign = `E2E November sale ${TAG}`;
    const id = sql(`
        WITH k AS (INSERT INTO campaigns (name, status, phone_number_id, template_name, template_language,
                                          template_params, audience, total_recipients, started_at)
                   SELECT '${campaign}', 'paused', id, 'e2e_promo', 'en_US', '{"body":["{{first_name}}"]}',
                          '{"all":true}', 3, now()
                   FROM phone_numbers WHERE phone_number_id = '9002${TAG}' RETURNING id),
             r AS (INSERT INTO campaign_recipients (campaign_id, contact_id, phone_e164)
                   SELECT k.id, c.id, c.phone_e164 FROM k, contacts c WHERE c.phone_e164 LIKE '+90555${TAG}_'),
             e AS (INSERT INTO campaign_events (campaign_id, kind) SELECT k.id, 'started' FROM k)
        SELECT id FROM k;`);
    sql(`INSERT INTO campaign_events (campaign_id, kind, detail) VALUES ('${id}', 'paused', '{"reason":"seeded by the test"}');`);
    expect(id).toMatch(/^[0-9a-f-]{36}$/);

    await signIn(page);
    await page.getByRole('link', { name: 'Campaigns' }).click();
    const row = page.getByTestId('campaign-row').filter({ hasText: campaign });
    await expect(row.getByTestId('campaign-status')).toHaveAttribute('data-status', 'paused');
    await row.getByRole('link', { name: campaign }).click();

    await expect(page.getByRole('heading', { name: campaign, level: 1 })).toBeVisible();
    await expect(page.getByTestId('campaign-status')).toHaveAttribute('data-status', 'paused');
    await expect(page.getByTestId('count-recipients')).toContainText('3');
    await expect(page.getByTestId('count-waiting')).toContainText('3');
    await expect(page.getByTestId('recipient-row')).toHaveCount(3);
    await expect(page.getByTestId('campaign-event')).toHaveCount(2);
    await expect(page.getByText('seeded by the test')).toBeVisible();

    // "Unknown" is explained, also for keyboard users.
    await page.getByRole('button', { name: 'What does “Unknown” mean?' }).focus();
    await expect(page.getByRole('tooltip')).toContainText('never re-sends');

    // Resume after confirming; the engine sends (the disabled account fails
    // each message at once) and completes, and the page follows over
    // realtime without a reload.
    await page.getByRole('button', { name: 'Resume' }).click();
    await page.getByRole('alertdialog').getByRole('button', { name: 'Resume' }).click();
    await expect(page.getByTestId('campaign-status')).toHaveAttribute('data-status', 'completed', { timeout: 60_000 });
    await expect(page.getByTestId('count-failed')).toContainText('2');
    await expect(page.getByTestId('count-skipped')).toContainText('1');
    await expect(page.getByTestId('campaign-event').filter({ hasText: 'Completed' })).toBeVisible();

    // Recipients carry their outcome and error code.
    await page.getByRole('button', { name: 'Refresh' }).click();
    await expect(page.getByTestId('recipient-row').filter({ hasText: 'ACCOUNT_DISABLED' })).toHaveCount(2);
    await page.getByRole('combobox', { name: 'Recipient state' }).click();
    await page.getByRole('option', { name: 'Skipped' }).click();
    await expect(page.getByTestId('recipient-row')).toHaveCount(1);
    await expect(page.getByTestId('recipient-row')).toContainText('missing first name');
    noErrors();
});

test('the wizard saves a draft with mapped variables, which can be deleted', async ({ page }) => {
    const noErrors = watchErrors(page);
    const name = `E2E draft ${TAG}`;
    await signIn(page);
    await page.goto('/campaigns/new');

    await page.getByLabel('Name', { exact: true }).fill(name);
    await page.getByRole('combobox', { name: 'Sender' }).click();
    await page.getByRole('option', { name: new RegExp(`555 ${TAG}`) }).click();
    await page.getByRole('button', { name: 'Next' }).click();

    await expect(page.getByRole('heading', { name: '2. Template' })).toBeVisible();
    await page.getByRole('combobox', { name: 'Template' }).click();
    await page.getByRole('option', { name: /e2e_promo/ }).click();
    await expect(page.getByTestId('template-preview')).toContainText('our sale starts today');
    await page.getByRole('button', { name: 'Next' }).click();

    // Placeholder chips insert contact data; a fallback covers empty names.
    const value = page.getByRole('textbox', { name: 'Body {{1}}' });
    await page.getByRole('button', { name: 'Next' }).click();
    await expect(page.getByText('Required.')).toBeVisible();
    await page.getByRole('button', { name: '{{first_name}}' }).click();
    await expect(value).toHaveValue('{{first_name}}');
    await expect(page.getByTestId('template-preview')).toContainText('Hi {{first_name}}, our sale');
    await page.getByLabel('If {{first_name}} is empty').fill('there');
    await page.getByRole('button', { name: 'Next' }).click();

    await expect(page.getByRole('heading', { name: '4. Audience' })).toBeVisible();
    await page.getByRole('radio', { name: 'All contacts' }).check();
    await expect(page.getByTestId('audience-estimate')).toContainText(/About \d+ contacts/);
    await page.getByRole('button', { name: 'Next' }).click();

    await expect(page.getByRole('heading', { name: '5. Review' })).toBeVisible();
    await expect(page.getByText('All contacts', { exact: false }).first()).toBeVisible();
    await page.getByRole('button', { name: 'Save draft' }).click();
    await expect(page).toHaveURL(/\/campaigns\/[0-9a-f-]{36}$/);
    await expect(page.getByTestId('campaign-status')).toHaveAttribute('data-status', 'draft');

    const params = sql(`SELECT template_params::text FROM campaigns WHERE name = '${name}'`);
    expect(JSON.parse(params)).toEqual({ body: ['{{first_name}}'], fallback: { first_name: 'there' } });

    await page.getByRole('button', { name: 'Delete' }).click();
    await page.getByRole('alertdialog').getByRole('button', { name: 'Delete' }).click();
    await expect(page).toHaveURL(/\/campaigns$/);
    expect(count(`SELECT count(*) FROM campaigns WHERE name = '${name}'`)).toBe(0);
    noErrors();
});
