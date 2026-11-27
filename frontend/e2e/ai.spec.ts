import { execFileSync } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { expect, test, type BrowserContext, type Page } from '@playwright/test';
import { totp } from './totp';

/**
 * Settings → AI and AI profiles with the fake provider (the E2E stack runs
 * with AI_FAKE_PROVIDER, see scripts/e2e.sh): connect a provider, refresh
 * its models, turn AI on, create a profile on a number and run the test
 * console. Runs after identity.spec.ts; the admin is invited directly.
 */

const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';
const RUN = Date.now().toString(36);
const ADMIN = {
    name: 'Ada AI',
    email: `ada+${RUN}@example.com`,
    password: 'Machines-answer-kindly-31',
};
const PROVIDER = `Fake ${RUN}`;
const PROFILE = `Sales ${RUN}`;

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
const digits = String(Date.now() % 1_000_000).padStart(6, '0');
const display = `+1 555 77${digits.slice(0, 2)} ${digits.slice(2)}`;
const errors: string[] = [];

test.beforeAll(async ({ browser }) => {
    const token = `msi_${randomBytes(32).toString('base64url')}`;
    sql(`INSERT INTO user_invitations (email, role_id, token_hash, expires_at)
         SELECT '${ADMIN.email}', id, sha256(convert_to('${token}', 'UTF8')), now() + interval '1 hour'
         FROM roles WHERE key = 'admin';`);
    account =
        sql(`INSERT INTO whatsapp_accounts (name, waba_id, access_token_enc, status)
        VALUES ('AI ${RUN}', '8${digits}1', '\\x00', 'disabled') RETURNING id;`);
    sql(`INSERT INTO phone_numbers (account_id, phone_number_id, display_phone_number, verified_name)
        VALUES ('${account}', '8${digits}2', '${display}', 'AI ${RUN}');`);

    context = await browser.newContext();
    page = await context.newPage();
    page.on('pageerror', (e) => errors.push(e.message));
    page.on('console', (m) => {
        if (m.type() === 'error') errors.push(m.text());
    });
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
    // Leave AI off and nothing of this run behind for later specs.
    sql(`DELETE FROM ai_profiles WHERE name = '${PROFILE}';
         DELETE FROM ai_providers WHERE name = '${PROVIDER}';
         DELETE FROM settings WHERE key = 'ai';
         DELETE FROM whatsapp_accounts WHERE id = '${account}';`);
    await context?.close();
});

test.afterEach(() => {
    expect(errors, 'browser errors').toEqual([]);
});

test('AI settings pages render', async () => {
    for (const [path, heading] of [
        ['/settings/ai', 'Company AI settings'],
        ['/settings/ai/providers', 'Providers'],
        ['/settings/ai/models', 'Model catalogue'],
        ['/ai-profiles', 'AI profiles'],
    ] as const) {
        const res = await page.goto(path);
        expect(res?.status(), path).toBeLessThan(400);
        await expect(page.locator('main h1').first()).toBeVisible();
        await expect(
            page.getByRole('heading', { name: heading }).first(),
        ).toBeVisible();
        await expect(page.getByText('Something went wrong')).toHaveCount(0);
    }
});

test('connect the fake provider and refresh its models', async () => {
    await page.goto('/settings/ai/providers');
    await page.getByRole('button', { name: 'Add provider' }).click();
    const dialog = page.getByRole('dialog', { name: 'Connect an AI provider' });
    await dialog.getByLabel('Provider').click();
    await page.getByRole('option', { name: 'Fake (tests)' }).click();
    await dialog.getByLabel('Name').fill(PROVIDER);
    const key = `fake-e2e-${RUN}-key`;
    await dialog.getByLabel('API key').fill(key);
    await dialog.getByRole('button', { name: 'Connect' }).click();
    await expect(dialog).toHaveCount(0);
    const row = page
        .getByTestId('ai-provider-row')
        .filter({ hasText: PROVIDER });
    await expect(row.getByTestId('ai-provider-status')).toHaveText('Working');
    // Only the last four characters are ever shown.
    await expect(row).toContainText(key.slice(-4));
    await expect(page.getByText(key)).toHaveCount(0);

    await row.getByRole('button', { name: 'Refresh models' }).click();
    await expect(page.getByText('2 models in the catalogue.')).toBeVisible();
    await page.goto('/settings/ai/models');
    const model = page
        .getByTestId('ai-model-row')
        .filter({ hasText: 'fake-small' });
    await expect(model).toBeVisible();
    await expect(model).toContainText('No price');
});

test('turn AI on with the fake model', async () => {
    await page.goto('/settings/ai');
    await page.getByRole('switch', { name: 'AI is on' }).click();
    await page.getByRole('combobox', { name: 'Provider' }).click();
    await page.getByRole('option', { name: PROVIDER, exact: false }).click();
    await page.getByRole('combobox', { name: 'Model' }).click();
    await page.getByRole('option', { name: /fake-small/ }).click();
    await page.getByRole('button', { name: 'Save', exact: true }).click();
    await expect(page.getByText('AI settings saved.')).toBeVisible();
});

test('create a profile on a number and run the test console', async () => {
    await page.goto('/ai-profiles');
    await page.getByRole('link', { name: 'New profile' }).click();
    await expect(page).toHaveURL(/\/ai-profiles\/new$/);
    await page.getByLabel('Profile name').fill(PROFILE);
    await page.getByLabel('WhatsApp number').click();
    await page.getByRole('option', { name: display, exact: false }).click();
    await page
        .getByLabel('Instructions (system prompt)')
        .fill('You are the sales assistant of Acme. Be brief.');
    await expect(page.getByTestId('ai-prompt-tokens')).toContainText('≈ 12');
    // Handoff on request is locked on (Meta policy).
    const onRequest = page.getByTestId('ai-handoff-on-request');
    await expect(onRequest).toBeDisabled();
    await expect(onRequest).toHaveAttribute('aria-checked', 'true');
    await page.getByTestId('ai-profile-save').click();
    await expect(page).toHaveURL(/\/ai-profiles\/[0-9a-f-]{36}$/);
    await expect(page.locator('main h1').first()).toHaveText(PROFILE);

    const consoleBox = page.getByTestId('ai-test-console');
    await consoleBox
        .getByLabel('Write a message…')
        .fill('Do you ship to Izmir?');
    await consoleBox.getByTestId('ai-test-run').click();
    await expect(consoleBox.getByTestId('ai-test-reply')).toHaveText(
        'Fake reply to: Do you ship to Izmir?',
    );
    await expect(consoleBox.getByTestId('ai-test-result')).toContainText(
        'fake-small',
    );

    await page.goto('/ai-profiles');
    const row = page.getByTestId('ai-profile-row').filter({ hasText: PROFILE });
    await expect(row).toContainText(display);
});
