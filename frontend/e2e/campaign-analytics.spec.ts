import { execFileSync } from 'node:child_process';
import { readFileSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';

/**
 * Campaign analytics (Phase 16C P3): the wizard's pre-launch analysis, the
 * detail page's analysis center, and the recipients table with a filtered
 * export. Needs identity.spec.ts first (the user reuses ari's password
 * hash). The channel, template and a finished campaign are seeded in the
 * database; its counters come from the database's own recompute.
 */

const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';
const USER = { email: 'nora@example.com', password: 'Another-strong-pass-73' };
const PERMISSIONS = [
    'channels.read',
    'templates.read',
    'campaigns.read',
    'campaigns.create',
    'campaigns.start',
    'campaigns.pause',
    'segments.read',
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
    await page.getByLabel('Password', { exact: true }).fill(USER.password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(
        page.getByRole('heading', { name: 'Welcome, Nora' }),
    ).toBeVisible();
}

function watchErrors(page: Page) {
    const errors: string[] = [];
    page.on('pageerror', (e) => errors.push(e.message));
    return () => expect(errors, 'browser errors').toEqual([]);
}

test.describe.configure({ mode: 'serial' });

let campaignId = '';

test.beforeAll(() => {
    const hash = sql(
        `SELECT password_hash FROM users WHERE email = 'ari@example.com'`,
    );
    expect(hash, 'identity.spec.ts must run first').toMatch(/^\$argon2id\$/);
    sql(`
        WITH r AS (INSERT INTO roles (key, name, rank) VALUES ('e2e_analyst', 'Analyst (E2E)', 25)
                   ON CONFLICT (key) DO UPDATE SET name = EXCLUDED.name RETURNING id),
             p AS (INSERT INTO role_permissions (role_id, permission_key)
                   SELECT r.id, k FROM r, unnest(ARRAY['${PERMISSIONS.join("','")}']) k ON CONFLICT DO NOTHING),
             u AS (INSERT INTO users (email, name, password_hash)
                   SELECT '${USER.email}', 'Nora Analyst', password_hash FROM users WHERE email = 'ari@example.com'
                   ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name RETURNING id)
        INSERT INTO user_roles (user_id, role_id) SELECT u.id, r.id FROM u, r ON CONFLICT DO NOTHING;`);
    // A disabled channel (nothing reaches Meta), a template with quick
    // replies, four contacts and a completed campaign: 3 sent, 2 delivered,
    // 1 read, 1 replied positive.
    sql(`
        WITH a AS (INSERT INTO whatsapp_accounts (name, waba_id, access_token_enc, status)
                   VALUES ('E2E Analytics ${TAG}', '9101${TAG}', '\\x00', 'disabled') RETURNING id),
             n AS (INSERT INTO phone_numbers (account_id, phone_number_id, display_phone_number, verified_name,
                                              quality_rating, messaging_limit)
                   SELECT id, '9102${TAG}', '+90 556 ${TAG}', 'Analytics', 'GREEN', 'TIER_1K' FROM a RETURNING id, account_id)
        INSERT INTO message_templates (account_id, meta_template_id, name, language, category, status, components)
        SELECT account_id, 'e2e-ana-${TAG}', 'e2e_survey', 'en_US', 'MARKETING', 'APPROVED',
               '[{"type":"BODY","text":"Did you like our new menu?"},
                 {"type":"BUTTONS","buttons":[{"type":"QUICK_REPLY","text":"Yes"},{"type":"QUICK_REPLY","text":"No"}]}]'::jsonb
        FROM n;
        INSERT INTO contacts (phone_e164, name, source, marketing_consent)
        SELECT '+90556${TAG}' || i, 'Analytics ' || i, 'import', 'opted_in' FROM generate_series(1, 4) i;`);
    campaignId = sql(`
        WITH k AS (INSERT INTO campaigns (name, status, phone_number_id, template_name, template_language,
                                          audience, total_recipients, started_at, finished_at, reaction_map)
                   SELECT 'E2E analytics ${TAG}', 'completed', id, 'e2e_survey', 'en_US', '{"all":true}', 4,
                          now() - interval '2 hours', now() - interval '1 hour', '{"Yes":"positive","No":"negative"}'
                   FROM phone_numbers WHERE phone_number_id = '9102${TAG}' RETURNING id, phone_number_id),
             m AS (INSERT INTO messages (phone_number_id, contact_id, direction, type, status, status_rank, source, campaign_id,
                                         sent_at, delivered_at, read_at)
                   SELECT k.phone_number_id, c.id, 'outbound', 'template',
                          CASE right(c.phone_e164, 1) WHEN '1' THEN 'read' WHEN '2' THEN 'delivered' ELSE 'sent' END, 4,
                          'campaign', k.id, now() - interval '90 minutes',
                          CASE WHEN right(c.phone_e164, 1) IN ('1', '2') THEN now() - interval '89 minutes' END,
                          CASE WHEN right(c.phone_e164, 1) = '1' THEN now() - interval '80 minutes' END
                   FROM k, contacts c WHERE c.phone_e164 IN ('+90556${TAG}1', '+90556${TAG}2', '+90556${TAG}3')
                   RETURNING id, contact_id, status, sent_at, delivered_at, read_at),
             r AS (INSERT INTO campaign_recipients (campaign_id, contact_id, phone_e164, state, message_id, msg_status,
                                                   attempted_at, sent_at, delivered_at, read_at, replied_at, reaction,
                                                   reaction_source, reply_payload)
                   SELECT k.id, c.id, c.phone_e164, CASE WHEN m.id IS NULL THEN 'cancelled' ELSE 'queued' END, m.id, m.status,
                          m.sent_at, m.sent_at, m.delivered_at, m.read_at,
                          CASE WHEN right(c.phone_e164, 1) = '1' THEN now() - interval '70 minutes' END,
                          CASE WHEN right(c.phone_e164, 1) = '1' THEN 'positive' END,
                          CASE WHEN right(c.phone_e164, 1) = '1' THEN 'quick_reply' END,
                          CASE WHEN right(c.phone_e164, 1) = '1' THEN 'Yes' END
                   FROM k CROSS JOIN contacts c LEFT JOIN m ON m.contact_id = c.id
                   WHERE c.phone_e164 LIKE '+90556${TAG}_' RETURNING campaign_id)
        SELECT DISTINCT campaign_id FROM r;`);
    sql(`SELECT campaign_counters_recompute('${campaignId}')`);
});

test('the wizard review shows the pre-launch analysis', async ({ page }) => {
    const noErrors = watchErrors(page);
    await signIn(page);
    await page.goto('/campaigns/new');
    await page.getByLabel('Name', { exact: true }).fill(`E2E forecast ${TAG}`);
    await page.getByRole('combobox', { name: 'Sender' }).click();
    await page.getByRole('option', { name: new RegExp(`556 ${TAG}`) }).click();
    await page.getByRole('button', { name: 'Next' }).click();
    await page.getByRole('combobox', { name: 'Template' }).click();
    await page.getByRole('option', { name: /e2e_survey/ }).click();
    await page.getByRole('button', { name: 'Next' }).click();
    await page.getByRole('button', { name: 'Next' }).click();
    await page.getByRole('radio', { name: 'All contacts' }).check();
    await page.getByRole('button', { name: 'Next' }).click();
    await expect(
        page.getByRole('heading', { name: '5. Review' }),
    ).toBeVisible();

    const forecast = page.getByTestId('campaign-forecast');
    await expect(forecast).toHaveAttribute('data-state', 'ready', {
        timeout: 15_000,
    });
    // One completed marketing campaign is not enough history: no rates
    // are invented.
    await expect(forecast.getByTestId('forecast-rates')).toHaveAttribute(
        'data-status',
        'insufficient_history',
    );
    await expect(forecast).toContainText('Not enough history');
    await expect(forecast.getByTestId('forecast-cost')).toContainText(
        'not in the rate card',
    );
    // The template's quick replies can be mapped.
    const map = page.getByTestId('reaction-map');
    await expect(map).toContainText('Yes');
    await expect(map).toContainText('No');
    noErrors();
});

test('the detail page shows the analysis center', async ({ page }) => {
    const noErrors = watchErrors(page);
    await signIn(page);
    await page.goto(`/campaigns/${campaignId}`);
    const center = page.getByRole('main').getByTestId('analysis-center');
    await expect(center).toBeVisible();
    const metrics = center.getByTestId('analysis-metrics');
    await expect(metrics.locator('[data-metric="delivered"]')).toContainText(
        '2',
    );
    await expect(
        metrics.locator('[data-metric="delivered"] [data-source="meta"]'),
    ).toBeVisible();
    await expect(metrics.locator('[data-metric="positive"]')).toContainText(
        '1',
    );
    await expect(
        metrics.locator('[data-metric="positive"] [data-source="system"]'),
    ).toBeVisible();
    for (const tab of [
        'cost',
        'quality',
        'breakdowns',
        'reports',
        'forecast',
    ]) {
        await center.getByTestId(`analysis-tab-${tab}`).click();
        if (tab === 'breakdowns')
            await expect(center.getByTestId('breakdowns')).toBeVisible();
    }
    await expect(center).toContainText('No forecast was stored');
    noErrors();
});

test('recipients filter and export', async ({ page }) => {
    const noErrors = watchErrors(page);
    await signIn(page);
    await page.goto(`/campaigns/${campaignId}`);
    const panel = page.getByRole('main').getByTestId('recipients-panel');
    await expect(panel.getByTestId('table-rows')).toContainText('of 4');
    await panel.getByTestId('recipients-status').click();
    await page.getByRole('menuitemcheckbox', { name: 'Delivered' }).click();
    await page.keyboard.press('Escape');
    await expect(panel.getByTestId('table-rows')).toContainText('of 2');
    await expect(panel.getByTestId('recipient-row')).toHaveCount(2);

    await panel.getByTestId('recipients-export').click();
    const [download] = await Promise.all([
        page.waitForEvent('download'),
        page.getByTestId('export-csv').click(),
    ]);
    const csv = readFileSync((await download.path())!, 'utf8');
    const lines = csv.trim().split('\n');
    expect(lines[0]).toContain('phone');
    expect(lines).toHaveLength(3);
    expect(csv).toContain(`+90556${TAG}1`);
    expect(csv).toContain(`+90556${TAG}2`);
    expect(csv).not.toContain(`+90556${TAG}3`);
    noErrors();
});
