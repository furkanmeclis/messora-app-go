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
    'channels.read',
    'channels.manage',
    'templates.read',
    'templates.manage',
    'campaigns.read',
    'campaigns.create',
    'campaigns.start',
    'campaigns.pause',
    'campaigns.delete',
    'segments.read',
    'contacts.read',
];
// Unique per run so the spec can be repeated on a kept stack.
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
            // Rows written here belong to tenant #1, the stack's only tenant
            // (the tenant_id column defaults to the session's app.tenant_id).
            'PGOPTIONS="-c app.tenant_id=00000000-0000-0000-0000-000000000001" psql -v ON_ERROR_STOP=1 -qtA -U "$POSTGRES_USER" -d "$POSTGRES_DB"',
        ],
        { cwd: '..', input: query, encoding: 'utf8' },
    ).trim();
}

const count = (query: string) => Number(sql(query));

async function signIn(page: Page) {
    await page.goto('/login');
    await page.getByLabel('E-mail').fill(USER.email);
    await page.getByLabel('Password', { exact: true }).fill(USER.password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(
        page.getByRole('heading', { name: 'Welcome, Mona' }),
    ).toBeVisible();
}

function watchErrors(page: Page) {
    const errors: string[] = [];
    page.on('pageerror', (e) => errors.push(e.message));
    return () => expect(errors, 'browser errors').toEqual([]);
}

test.describe.configure({ mode: 'serial' });

test.beforeAll(() => {
    const hash = sql(
        `SELECT password_hash FROM users WHERE email = 'ari@example.com'`,
    );
    expect(
        hash,
        'identity.spec.ts must run first (it creates ari@example.com)',
    ).toMatch(/^\$argon2id\$/);
    // A role with exactly what these screens need, so the test does not
    // depend on the owner's second factor.
    sql(`
        WITH r AS (INSERT INTO roles (key, name, rank) VALUES ('e2e_marketing', 'Marketing (E2E)', 25)
                   ON CONFLICT (key) DO UPDATE SET name = EXCLUDED.name RETURNING id),
             p AS (INSERT INTO role_permissions (role_id, permission_key)
                   SELECT r.id, k FROM r, unnest(ARRAY['${PERMISSIONS.join("','")}']) k ON CONFLICT DO NOTHING),
             u AS (INSERT INTO users (email, name, password_hash, email_verified_at)
                   SELECT '${USER.email}', 'Mona Marketing', password_hash, now() FROM users WHERE email = 'ari@example.com'
                   ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name RETURNING id)
        INSERT INTO tenant_memberships (user_id, role_id) SELECT u.id, r.id FROM u, r ON CONFLICT DO NOTHING;`);
});

test('channels: the connect form validates input and shows why Meta refused', async ({
    page,
}) => {
    const noErrors = watchErrors(page);
    await signIn(page);
    await page.getByRole('link', { name: 'Channels' }).click();
    await expect(
        page.getByRole('heading', { name: 'Channels', level: 1 }),
    ).toBeVisible();
    if (count('SELECT count(*) FROM whatsapp_accounts') === 0) {
        await expect(
            page.getByText('No WhatsApp account connected'),
        ).toBeVisible();
    }

    // The webhook values moved to Settings → WhatsApp / Meta.
    await expect(
        page.getByRole('link', { name: 'Open the Meta connection' }),
    ).toHaveAttribute('href', '/settings/meta');

    await page.getByRole('button', { name: 'Connect account' }).click();
    const dialog = page.getByRole('dialog');
    const name = dialog.getByLabel('Name', { exact: true });
    const waba = dialog.getByLabel('WhatsApp Business Account ID');
    const submit = dialog.getByRole('button', { name: 'Connect', exact: true });

    // The browser checks required fields and formats before anything is sent.
    await submit.click();
    await expect
        .poll(() =>
            name.evaluate(
                (el) => (el as HTMLInputElement).validity.valueMissing,
            ),
        )
        .toBe(true);
    await name.fill('E2E Brand');
    await waba.fill('not-a-number');
    await dialog.getByLabel('Access token').fill('EAAG' + 'x'.repeat(40));
    await submit.click();
    await expect
        .poll(() =>
            waba.evaluate(
                (el) => (el as HTMLInputElement).validity.patternMismatch,
            ),
        )
        .toBe(true);

    // A well-formed but made-up token: Meta (or the network) refuses it and
    // the API's message is shown in the dialog; nothing is stored. The App
    // ID and App secret are required too.
    await waba.fill(`9009${TAG}`);
    await dialog.getByLabel('App ID', { exact: true }).fill('1234567890');
    await dialog
        .getByLabel('App secret', { exact: true })
        .fill('made-up-app-secret');
    await submit.click();
    await expect(
        dialog.getByRole('alert').filter({ hasText: /Meta/ }),
    ).toBeVisible({ timeout: 75_000 });
    await expect(name).toHaveValue('E2E Brand');
    expect(
        count(
            `SELECT count(*) FROM whatsapp_accounts WHERE waba_id = '9009${TAG}'`,
        ),
    ).toBe(0);
    noErrors();
});

test('templates and campaigns start empty; the wizard needs a phone number', async ({
    page,
}) => {
    const noErrors = watchErrors(page);
    await signIn(page);

    await page.getByRole('link', { name: 'Templates' }).click();
    await expect(
        page.getByRole('heading', { name: 'Templates', level: 1 }),
    ).toBeVisible();
    if (count('SELECT count(*) FROM message_templates') === 0) {
        await expect(page.getByText('No templates yet')).toBeVisible();
    }
    await page.getByRole('combobox', { name: 'Status' }).click();
    await page.getByRole('option', { name: 'Rejected' }).click();
    await expect(page).toHaveURL(/status=REJECTED/);
    if (
        count(
            `SELECT count(*) FROM message_templates WHERE status = 'REJECTED'`,
        ) === 0
    ) {
        await expect(
            page.getByText('No templates match these filters'),
        ).toBeVisible();
    }

    await page
        .locator('[data-sidebar="sidebar"]')
        .getByRole('link', { name: 'Campaigns' })
        .click();
    await expect(
        page.getByRole('heading', { name: 'Campaigns', level: 1 }),
    ).toBeVisible();
    if (count('SELECT count(*) FROM campaigns') === 0) {
        await expect(page.getByText('No campaigns yet')).toBeVisible();
    }

    await page.getByRole('link', { name: 'New campaign' }).click();
    await expect(
        page.getByRole('heading', { name: '1. Basics' }),
    ).toBeVisible();
    await page.getByRole('button', { name: 'Next' }).click();
    await expect(page.getByText('Give the campaign a name.')).toBeVisible();
    await page.getByLabel('Name', { exact: true }).fill('Without a number');
    if (count('SELECT count(*) FROM phone_numbers') === 0) {
        await expect(page.getByTestId('no-phone-number')).toContainText(
            'No phone number connected',
        );
        await page.getByRole('button', { name: 'Next' }).click();
        // The wizard stays on the first step.
        await expect(
            page.getByRole('heading', { name: '1. Basics' }),
        ).toBeVisible();
    } else {
        await expect(
            page.getByRole('combobox', { name: 'Sender' }),
        ).toBeVisible();
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

test('a paused campaign resumes and its dashboard follows it live', async ({
    page,
}) => {
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
    sql(
        `INSERT INTO campaign_events (campaign_id, kind, detail) VALUES ('${id}', 'paused', '{"reason":"seeded by the test"}');`,
    );
    expect(id).toMatch(/^[0-9a-f-]{36}$/);

    await signIn(page);
    await page
        .locator('[data-sidebar="sidebar"]')
        .getByRole('link', { name: 'Campaigns' })
        .click();
    const row = page.getByTestId('campaign-row').filter({ hasText: campaign });
    await expect(row.getByTestId('campaign-status')).toHaveAttribute(
        'data-status',
        'paused',
    );
    await row.getByRole('link', { name: campaign }).click();

    await expect(
        page.getByRole('heading', { name: campaign, level: 1 }),
    ).toBeVisible();
    await expect(page.getByTestId('campaign-status')).toHaveAttribute(
        'data-status',
        'paused',
    );
    await expect(page.getByTestId('count-recipients')).toContainText('3');
    await expect(page.getByTestId('count-waiting')).toContainText('3');
    // One recipients table, paged by its own pager (no "load more").
    const recipients = page.getByTestId('recipients-panel');
    await expect(recipients.getByTestId('recipient-row')).toHaveCount(3);
    await expect(recipients.getByTestId('table-rows')).toContainText('of 3');
    await expect(page.getByRole('button', { name: 'Load more' })).toHaveCount(
        0,
    );
    await expect(page.getByTestId('campaign-event')).toHaveCount(2);
    await expect(page.getByText('seeded by the test')).toBeVisible();

    // "Unknown" is explained, also for keyboard users.
    await page
        .getByRole('button', { name: 'What does “Unknown” mean?' })
        .focus();
    await expect(page.getByRole('tooltip')).toContainText('never re-sends');

    // Resume after confirming; the engine sends (the disabled account fails
    // each message at once) and completes, and the page follows over
    // realtime without a reload.
    await page.getByTestId('resume-open').click();
    // The pre-resume summary counts what is left, consent checked again.
    const resume = page.getByRole('dialog');
    await expect(resume.getByTestId('preview-remaining')).toHaveText('3');
    await resume.getByTestId('resume-confirm').click();
    await expect(page.getByTestId('campaign-status')).toHaveAttribute(
        'data-status',
        'completed',
        { timeout: 60_000 },
    );
    await expect(page.getByTestId('count-failed')).toContainText('2');
    await expect(page.getByTestId('count-skipped')).toContainText('1');
    await expect(
        page.getByTestId('campaign-event').filter({ hasText: 'Completed' }),
    ).toBeVisible();

    // Recipients carry one status and one reason: the class once, then
    // the error code (the table follows realtime; Refresh re-reads now).
    await recipients.getByRole('button', { name: 'Refresh' }).click();
    const failed = recipients
        .getByTestId('recipient-row')
        .filter({ hasText: 'ACCOUNT_DISABLED' });
    await expect(failed).toHaveCount(2);
    await expect(
        failed.first().locator('[data-status="failed"]'),
    ).toBeVisible();
    await expect(failed.first().getByTestId('recipient-reason')).toHaveCount(1);
    await recipients.getByTestId('recipients-status').click();
    await page
        .getByRole('menuitemcheckbox', { name: 'Blocked', exact: true })
        .click();
    await page.keyboard.press('Escape');
    await expect(recipients.getByTestId('recipient-row')).toHaveCount(1);
    await expect(recipients.getByTestId('recipient-row')).toContainText(
        'missing first name',
    );
    noErrors();
});

test('image-header templates: the wizard takes the image and keeps it on edit', async ({
    page,
}) => {
    const noErrors = watchErrors(page);
    // The channel was seeded by the previous test.
    sql(`
        INSERT INTO message_templates (account_id, meta_template_id, name, language, category, status, components)
        SELECT account_id, 'e2e-photo-${TAG}', 'e2e_photo', 'tr', 'MARKETING', 'APPROVED',
               '[{"type":"HEADER","format":"IMAGE"},{"type":"BODY","text":"Merhaba {{1}}!"},
                 {"type":"BUTTONS","buttons":[{"type":"QUICK_REPLY","text":"Dur"},{"type":"URL","text":"Bak","url":"https://shop.test/{{1}}"}]}]'::jsonb
        FROM phone_numbers WHERE phone_number_id = '9002${TAG}';`);
    const name = `E2E photo ${TAG}`;
    await signIn(page);
    await page.goto('/campaigns/new');
    await page.getByLabel('Name', { exact: true }).fill(name);
    await page.getByRole('combobox', { name: 'Sender' }).click();
    await page.getByRole('option', { name: new RegExp(`555 ${TAG}`) }).click();
    await page.getByRole('button', { name: 'Next' }).click();
    await page.getByRole('combobox', { name: 'Template' }).click();
    await page.getByRole('option', { name: /e2e_photo/ }).click();
    await page.getByRole('button', { name: 'Next' }).click();

    // Nothing filled: the image, the variable and the button are asked for.
    await page.getByRole('button', { name: 'Next' }).click();
    await expect(page.getByText('Add the header image.')).toBeVisible();
    await page.getByRole('radio', { name: 'Link' }).click();
    await page.getByLabel('Link').fill('https://cdn.example.com/kampanya.jpg');
    await page.getByRole('combobox', { name: 'Body {{1}}' }).click();
    await page.getByRole('option', { name: 'Full name' }).click();
    await page.getByRole('radio', { name: 'Fixed text' }).last().click();
    await page
        .getByRole('textbox', { name: 'Button 2: end of the address' })
        .fill('yaz-indirimi');
    // The panel does not load images from other sites (its content
    // security policy): the preview names the link instead.
    await expect(page.getByTestId('template-preview')).toContainText(
        'https://cdn.example.com/kampanya.jpg',
    );
    await page.getByRole('button', { name: 'Next' }).click();
    await page.getByRole('radio', { name: 'All contacts' }).check();
    await page.getByRole('button', { name: 'Next' }).click();
    await page.getByRole('button', { name: 'Save draft' }).click();
    await expect(page).toHaveURL(/\/campaigns\/[0-9a-f-]{36}$/);

    const params = JSON.parse(
        sql(
            `SELECT template_params::text FROM campaigns WHERE name = '${name}'`,
        ),
    );
    expect(params).toEqual({
        body: ['{{name}}'],
        buttons: ['', 'yaz-indirimi'],
        header_media: {
            type: 'image',
            link: 'https://cdn.example.com/kampanya.jpg',
            filename: undefined,
        },
    });

    // Editing keeps the image (it used to be dropped).
    const id = page.url().split('/').pop();
    await page.goto(`/campaigns/${id}/edit`);
    await page.getByRole('button', { name: 'Next' }).click();
    await page.getByRole('button', { name: 'Next' }).click();
    await expect(page.getByTestId('param-media')).toContainText(
        'https://cdn.example.com/kampanya.jpg',
    );
    noErrors();
});

test('the wizard saves a draft with mapped variables, which can be deleted', async ({
    page,
}) => {
    const noErrors = watchErrors(page);
    const name = `E2E draft ${TAG}`;
    await signIn(page);
    await page.goto('/campaigns/new');

    await page.getByLabel('Name', { exact: true }).fill(name);
    await page.getByRole('combobox', { name: 'Sender' }).click();
    await page.getByRole('option', { name: new RegExp(`555 ${TAG}`) }).click();
    await page.getByRole('button', { name: 'Next' }).click();

    await expect(
        page.getByRole('heading', { name: '2. Template' }),
    ).toBeVisible();
    await page.getByRole('combobox', { name: 'Template' }).click();
    await page.getByRole('option', { name: /e2e_promo/ }).click();
    await expect(page.getByTestId('template-preview')).toContainText(
        'our sale starts today',
    );
    await page.getByRole('button', { name: 'Next' }).click();

    // Each variable maps to a contact field; a value covers empty names.
    await page.getByRole('button', { name: 'Next' }).click();
    await expect(page.getByText('Body {{1}} needs a value.')).toBeVisible();
    await page.getByRole('combobox', { name: 'Body {{1}}' }).click();
    await page.getByRole('option', { name: 'First name' }).click();
    // The preview shows a real contact's value, not the placeholder.
    await expect(page.getByTestId('template-preview')).toContainText(
        'our sale starts today',
    );
    await expect(page.getByTestId('template-preview')).not.toContainText(
        '{{1}}',
    );
    await page.getByLabel('If empty: First name').fill('there');
    await page.getByRole('button', { name: 'Next' }).click();

    await expect(
        page.getByRole('heading', { name: '4. Audience' }),
    ).toBeVisible();
    await page.getByRole('radio', { name: 'All contacts' }).check();
    await expect(page.getByTestId('audience-estimate')).toContainText(
        /About \d+ contacts/,
    );
    await page.getByRole('button', { name: 'Next' }).click();

    await expect(
        page.getByRole('heading', { name: '5. Review' }),
    ).toBeVisible();
    await expect(
        page.getByText('All contacts', { exact: false }).first(),
    ).toBeVisible();
    await page.getByRole('button', { name: 'Save draft' }).click();
    await expect(page).toHaveURL(/\/campaigns\/[0-9a-f-]{36}$/);
    await expect(page.getByTestId('campaign-status')).toHaveAttribute(
        'data-status',
        'draft',
    );

    const params = sql(
        `SELECT template_params::text FROM campaigns WHERE name = '${name}'`,
    );
    expect(JSON.parse(params)).toEqual({
        body: ['{{first_name}}'],
        fallback: { first_name: 'there' },
    });

    await page.getByRole('button', { name: 'Delete' }).click();
    await page
        .getByRole('alertdialog')
        .getByRole('button', { name: 'Delete' })
        .click();
    await expect(page).toHaveURL(/\/campaigns$/);
    expect(count(`SELECT count(*) FROM campaigns WHERE name = '${name}'`)).toBe(
        0,
    );
    noErrors();
});

test('the review step shows who consent keeps from the campaign', async ({
    page,
}) => {
    const noErrors = watchErrors(page);
    // The three contacts seeded with the channel, under a tag of their own
    // so the audience is exactly them: one opted out of marketing, the
    // others opted in (so the result does not depend on the company's
    // policy for unknown consent).
    const tag = `e2e-consent-${TAG}`;
    sql(`
        WITH t AS (INSERT INTO tags (name) VALUES ('${tag}') RETURNING id)
        INSERT INTO contact_tags (contact_id, tag_id)
        SELECT c.id, t.id FROM t, contacts c WHERE c.phone_e164 LIKE '+90555${TAG}_';
        UPDATE contacts SET marketing_consent = 'opted_in', marketing_consent_at = now(),
                            marketing_consent_source = 'manual'
        WHERE phone_e164 LIKE '+90555${TAG}_';
        UPDATE contacts SET marketing_consent = 'opted_out', marketing_consent_at = now(),
                            marketing_consent_source = 'manual', marketing_consent_reason = 'E2E'
        WHERE phone_e164 = '+90555${TAG}2';`);

    await signIn(page);
    await page.goto('/campaigns/new');
    await page.getByLabel('Name', { exact: true }).fill(`E2E consent ${TAG}`);
    await page.getByRole('combobox', { name: 'Sender' }).click();
    await page.getByRole('option', { name: new RegExp(`555 ${TAG}`) }).click();
    await page.getByRole('button', { name: 'Next' }).click();
    await page.getByRole('combobox', { name: 'Template' }).click();
    await page.getByRole('option', { name: /e2e_promo/ }).click();
    await page.getByRole('button', { name: 'Next' }).click();
    await page.getByRole('combobox', { name: 'Body {{1}}' }).click();
    await page.getByRole('option', { name: 'First name' }).click();
    await page.getByLabel('If empty: First name').fill('there');
    await page.getByRole('button', { name: 'Next' }).click();

    await expect(
        page.getByRole('heading', { name: '4. Audience' }),
    ).toBeVisible();
    await page.getByRole('radio', { name: 'A tag' }).check();
    await page.getByRole('combobox', { name: 'Tag', exact: true }).click();
    await page.getByRole('option', { name: new RegExp(tag) }).click();
    await page.getByRole('button', { name: 'Next' }).click();

    await expect(
        page.getByRole('heading', { name: '5. Review' }),
    ).toBeVisible();
    const preflight = page.getByTestId('campaign-preflight');
    await expect(preflight).toHaveAttribute('data-state', 'ready');
    await expect(page.getByTestId('preflight-summary')).toContainText(
        /Your audience has 3 people; 1 said they don.t want messages\./,
    );
    await expect(page.getByTestId('preflight-blocked')).toHaveAttribute(
        'data-count',
        '1',
    );
    const optedOut = page.getByTestId('preflight-reason-opted_out');
    await expect(optedOut).toHaveAttribute('data-count', '1');
    await expect(optedOut).toContainText('Always blocked');
    await expect(page.getByTestId('preflight-eligible')).toHaveAttribute(
        'data-count',
        '2',
    );
    await expect(page.getByTestId('preflight-eligible')).toContainText(
        '2 will receive it',
    );
    // A hard block cannot be overridden from here.
    await expect(preflight.getByRole('button')).toHaveCount(0);
    noErrors();
});

test('unreached customers of a finished campaign are targeted again, with a priority', async ({
    page,
}) => {
    const noErrors = watchErrors(page);
    // A finished campaign on the seeded channel: contact 1 failed with a
    // temporary error, contact 2 was blocked by consent (never selectable),
    // contact 3 was never sent (the campaign was cancelled).
    const first = `E2E first wave ${TAG}`;
    sql(`
        UPDATE contacts SET marketing_consent = 'opted_in', marketing_consent_at = now(),
                            marketing_consent_source = 'manual', opted_out_at = NULL
        WHERE phone_e164 IN ('+90555${TAG}1', '+90555${TAG}3');
        WITH k AS (INSERT INTO campaigns (name, status, phone_number_id, template_name, template_language,
                                          audience, total_recipients, started_at, finished_at)
                   SELECT '${first}', 'cancelled', id, 'e2e_promo', 'en_US', '{"all":true}', 3, now(), now()
                   FROM phone_numbers WHERE phone_number_id = '9002${TAG}' RETURNING id, phone_number_id),
             m AS (INSERT INTO messages (phone_number_id, contact_id, direction, type, status, status_rank, source,
                                         campaign_id, failure_class, error_code)
                   SELECT k.phone_number_id, c.id, 'outbound', 'template', 'failed', 6, 'campaign', k.id, 'temporary', '131000'
                   FROM k, contacts c WHERE c.phone_e164 = '+90555${TAG}1' RETURNING id, contact_id)
        INSERT INTO campaign_recipients (campaign_id, contact_id, phone_e164, state, skip_reason, message_id)
        SELECT k.id, c.id, c.phone_e164,
               CASE c.phone_e164 WHEN '+90555${TAG}1' THEN 'queued' WHEN '+90555${TAG}2' THEN 'skipped' ELSE 'cancelled' END,
               CASE c.phone_e164 WHEN '+90555${TAG}2' THEN 'opted_out' END,
               (SELECT m.id FROM m WHERE m.contact_id = c.id)
        FROM k, contacts c WHERE c.phone_e164 LIKE '+90555${TAG}_';`);

    const name = `E2E second wave ${TAG}`;
    await signIn(page);
    await page.goto('/campaigns/new');
    await page.getByLabel('Name', { exact: true }).fill(name);
    await page.getByRole('combobox', { name: 'Sender' }).click();
    await page.getByRole('option', { name: new RegExp(`555 ${TAG}`) }).click();
    await page.getByRole('button', { name: 'Next' }).click();
    await page.getByRole('combobox', { name: 'Template' }).click();
    await page.getByRole('option', { name: /e2e_promo/ }).click();
    await page.getByRole('button', { name: 'Next' }).click();
    await page.getByRole('combobox', { name: 'Body {{1}}' }).click();
    await page.getByRole('option', { name: 'First name' }).click();
    await page.getByLabel('If empty: First name').fill('there');
    await page.getByRole('button', { name: 'Next' }).click();

    await expect(
        page.getByRole('heading', { name: '4. Audience' }),
    ).toBeVisible();
    await page.getByRole('radio', { name: 'Not reached before' }).check();
    await page.getByRole('combobox', { name: 'Earlier campaign' }).click();
    await page.getByRole('option', { name: first }).click();

    // Live summary per reason; hard blocks are listed but not selectable.
    const prev = page.getByTestId('previous-audience');
    await expect(prev.getByTestId('class-count-temporary')).toHaveText('1');
    await expect(prev.getByTestId('class-count-never_sent')).toHaveText('1');
    await expect(prev).toContainText('Blocked by consent (1)');
    await expect(page.getByTestId('previous-summary')).toContainText(
        '2 will be sent',
    );
    await prev.getByTestId('class-never_sent').click();
    await expect(page.getByTestId('previous-summary')).toContainText(
        '1 will be sent',
    );
    await page.getByRole('button', { name: 'Next' }).click();

    await expect(
        page.getByRole('heading', { name: '5. Review' }),
    ).toBeVisible();
    await expect(page.getByText(`Not reached by ${first}`)).toBeVisible();
    await page.getByTestId('wizard-priority').click();
    await page.getByRole('option', { name: 'High' }).click();
    await page.getByRole('button', { name: 'Save draft' }).click();
    await expect(page).toHaveURL(/\/campaigns\/[0-9a-f-]{36}$/);

    const stored = JSON.parse(
        sql(
            `SELECT json_build_object('audience', audience, 'priority', priority)::text FROM campaigns WHERE name = '${name}'`,
        ),
    ) as {
        priority: string;
        audience: { previous_campaign: { classes: string[] } };
    };
    expect(stored.priority).toBe('high');
    expect(stored.audience.previous_campaign.classes.sort()).toEqual([
        'ecosystem_limit',
        'temporary',
    ]);

    // The priority can be changed on the detail page.
    const engine = page.getByTestId('campaign-engine');
    await expect(engine.getByTestId('priority-select')).toContainText('High');
    await engine.getByTestId('priority-select').click();
    await page.getByRole('option', { name: 'Low' }).click();
    await expect(page.getByText('Priority changed.')).toBeVisible();
    await expect
        .poll(() =>
            sql(`SELECT priority FROM campaigns WHERE name = '${name}'`),
        )
        .toBe('low');
    expect(
        count(
            `SELECT count(*) FROM audit_logs a JOIN campaigns c ON c.id = a.entity_id
             WHERE c.name = '${name}' AND a.action = 'campaign.priority_change'`,
        ),
    ).toBe(1);

    // The list shows it too.
    await page.goto('/campaigns');
    await expect(
        page
            .getByTestId('campaign-row')
            .filter({ hasText: name })
            .getByTestId('campaign-priority'),
    ).toHaveText('Low');
    noErrors();
});
