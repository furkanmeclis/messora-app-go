import { execFileSync } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { expect, test, type BrowserContext, type Page } from '@playwright/test';
import { totp } from './totp';

/**
 * AI customer intelligence and AI usage (Phase 16C P7). Runs after
 * identity.spec.ts; the admin is invited directly. The contact the AI built
 * from an unknown number and the AI usage rows are seeded in the database
 * (the AI turns that produce them are covered by the integration tests).
 */

const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';
const RUN = Date.now().toString(36);
const ADMIN = {
    name: 'Ilgaz Intel',
    email: `ilgaz+${RUN}@example.com`,
    password: 'Profiles-built-by-ai-58',
};

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

test.describe.configure({ mode: 'serial' });

let context: BrowserContext;
let page: Page;
let contact = '';
const digits = String(Date.now() % 1_000_000).padStart(6, '0');
const phone = `+90544${digits}3`;
const NAME = `Deniz Yeni ${RUN}`;

test.beforeAll(async ({ browser }) => {
    const token = `msi_${randomBytes(32).toString('base64url')}`;
    sql(`INSERT INTO user_invitations (email, role_id, token_hash, expires_at)
         SELECT '${ADMIN.email}', id, sha256(convert_to('${token}', 'UTF8')), now() + interval '1 hour'
         FROM roles WHERE key = 'admin';`);
    contact = sql(`
        INSERT INTO contacts (phone_e164, name, profile_name, attributes, created_by_inbound_at, profile_status, profile_progress)
        VALUES ('${phone}', '${NAME}', 'Deniz', '{"company": "Acme ${RUN}"}', now(), 'complete',
                '{"required": 2, "filled": 2, "missing": []}')
        RETURNING id;`);
    sql(`INSERT INTO contact_field_sources (contact_id, field, source, verified) VALUES
             ('${contact}', 'name', 'ai', false), ('${contact}', 'attr:company', 'ai', false);
         INSERT INTO ai_usage (provider_kind, model, purpose, contact_id, input_tokens, output_tokens, cost, currency, latency_ms, status)
         VALUES ('fake', 'fake-small', 'reply', '${contact}', 1200, 300, 0.25, 'USD', 850, 'ok'),
                ('fake', 'fake-small', 'classify', '${contact}', 200, 20, 0.01, 'USD', 300, 'ok');`);

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
    sql(`DELETE FROM ai_usage WHERE contact_id = '${contact}';
         DELETE FROM contacts WHERE id = '${contact}';`);
    await context?.close();
});

test('the contact shows what the AI collected, and a person verifies it', async () => {
    await page.goto(`/contacts/${contact}`);
    const main = page.getByRole('main');
    const card = main.getByTestId('ai-profile-card');
    await expect(
        card.getByText('Collected by the AI', { exact: true }),
    ).toBeVisible();
    await expect(card.getByText('Complete', { exact: true })).toBeVisible();
    await expect(card.getByText('2 of 2 required fields')).toBeVisible();
    const name = card.getByTestId('ai-field-name');
    await expect(name.getByText(NAME)).toBeVisible();
    await expect(
        name.getByText('Collected by the AI, not verified'),
    ).toBeVisible();
    await name.getByRole('button', { name: 'Verify' }).click();
    await expect(name.getByText('Verified', { exact: true })).toBeVisible();
    await expect(name.getByRole('button', { name: 'Verify' })).toHaveCount(0);
    // The company is still unverified.
    await expect(
        card
            .getByTestId('ai-field-attr:company')
            .getByText('Collected by the AI, not verified'),
    ).toBeVisible();
    expect(
        sql(
            `SELECT verified FROM contact_field_sources WHERE contact_id = '${contact}' AND field = 'name';`,
        ),
    ).toBe('t');
});

test('the contacts list filters contacts the AI created', async () => {
    await page.goto('/contacts?ai_profile=true');
    const row = page.getByTestId('contact-row-name').filter({ hasText: NAME });
    await expect(row).toBeVisible();
    await expect(row.getByTestId('contact-ai-badge')).toHaveText('AI');
});

test('the AI usage page renders the KPIs, the daily cost and the breakdowns', async () => {
    await page.goto('/settings/ai/usage');
    await expect(page.getByRole('link', { name: 'Usage' })).toBeVisible();
    const usage = page.getByRole('main').getByTestId('ai-usage');
    await expect(
        usage.getByRole('heading', { name: 'AI usage' }),
    ).toBeVisible();
    await expect(usage.getByTestId('ai-usage-requests')).toContainText(/\d/);
    await expect(usage.getByTestId('ai-usage-cost')).toContainText('$');
    await expect(usage.getByTestId('ai-usage-chart')).toBeVisible();
    await expect(
        usage.getByTestId('ai-usage-by-model').getByText('fake/fake-small'),
    ).toBeVisible();
    await expect(
        usage
            .getByTestId('ai-usage-by-campaign')
            .getByText('Outside campaigns'),
    ).toBeVisible();
    await expect(
        usage.getByTestId('ai-usage-by-purpose').getByText('Classification'),
    ).toBeVisible();
    await page.getByRole('radio', { name: 'Last 7 days' }).click();
    await expect(page).toHaveURL(/range=7/);
    await expect(usage.getByTestId('ai-usage-chart')).toBeVisible();
});
