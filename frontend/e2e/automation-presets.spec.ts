import { execFileSync } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { expect, test, type BrowserContext, type Page } from '@playwright/test';

/**
 * Preset library: a manager opens "Ready-made automations", installs one
 * with a click and lands on its canvas (disabled, steps on the canvas). An
 * AI preset shows the "AI is off" warning on its AI node while the
 * company's AI is off, and a preset that needs a value marks the step and
 * keeps the automation from being turned on.
 *
 * Runs after identity.spec.ts (see playwright.config.ts); signs in as a
 * fresh manager invited straight through the database.
 */

const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';
const RUN = Date.now().toString(36);
const MANAGER = {
    name: 'Pia Preset',
    email: `pia+${RUN}@example.com`,
    password: 'Ready-made-flows-42',
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
const errors: string[] = [];
const installed: string[] = [];

test.beforeAll(async ({ browser }) => {
    const token = `msi_${randomBytes(32).toString('base64url')}`;
    sql(`INSERT INTO user_invitations (email, role_id, token_hash, expires_at)
         SELECT '${MANAGER.email}', id, sha256(convert_to('${token}', 'UTF8')), now() + interval '1 hour'
         FROM roles WHERE key = 'manager';`);
    // The company's AI is off (the default; another spec may have turned
    // it on).
    sql(
        `UPDATE settings SET value = jsonb_set(value, '{enabled}', 'false') WHERE key = 'ai';`,
    );

    context = await browser.newContext({
        viewport: { width: 1680, height: 1050 },
    });
    page = await context.newPage();
    page.on('pageerror', (e) => errors.push(e.message));
    page.on('console', (m) => {
        if (m.type() === 'error') errors.push(m.text());
    });
    await page.goto(`/invite?token=${token}`);
    await expect(page.getByText('You were invited as Manager.')).toBeVisible();
    await page.getByLabel('Your name').fill(MANAGER.name);
    await page.getByLabel('Password').fill(MANAGER.password);
    await page.getByRole('button', { name: 'Join' }).click();
    await expect(
        page.getByRole('heading', { name: 'Welcome, Pia' }),
    ).toBeVisible();
});

test.afterAll(async () => {
    // Leave nothing that could react to later specs' events.
    if (installed.length) {
        sql(
            `DELETE FROM automations WHERE id IN (${installed.map((id) => `'${id}'`).join(', ')});`,
        );
    }
    await context?.close();
});

test.afterEach(() => {
    expect(errors, 'browser errors').toEqual([]);
});

const canvas = () => page.getByTestId('automation-canvas');
const node = (type: string) =>
    canvas().locator(`[data-testid="canvas-node"][data-step-type="${type}"]`);

/** Installs a preset from the gallery and returns the new automation id. */
async function install(key: string): Promise<string> {
    await page.goto('/automations');
    await page.getByTestId('presets-link').filter({ visible: true }).click();
    await expect(page).toHaveURL(/\/automations\/presets$/);
    await expect(
        page.getByRole('heading', { name: 'Ready-made automations' }),
    ).toBeVisible();
    await page.getByTestId(`install-preset-${key}`).click();
    await expect(page).toHaveURL(/\/automations\/[0-9a-f-]{36}$/);
    const id = page.url().split('/').at(-1)!;
    installed.push(id);
    await expect(node('trigger')).toBeVisible();
    return id;
}

test('a preset is installed from the gallery and opens on the canvas', async () => {
    const id = await install('welcome');
    await expect(node('reply')).toBeVisible();
    await expect(node('add_tag')).toBeVisible();
    await expect(node('reply')).toContainText('Merhaba');
    // Installed disabled, as a normal automation.
    expect(
        sql(`SELECT active, preset_key FROM automations WHERE id = '${id}';`),
    ).toBe('f|welcome');
    await expect(page.getByTestId('ai-off-badge')).toHaveCount(0);
});

test('an AI node warns while the company AI is off', async () => {
    await install('collect_customer_info');
    await expect(node('ai_extract_fields')).toBeVisible();
    const badge = node('ai_extract_fields').getByTestId('ai-off-badge');
    await expect(badge).toBeVisible();
    await expect(
        badge.getByRole('link', { name: 'AI settings' }),
    ).toHaveAttribute('href', '/settings/ai');
    // Non-AI nodes carry no warning.
    await expect(node('add_tag').getByTestId('ai-off-badge')).toHaveCount(0);
});

test('a preset that needs a value marks the step and cannot be enabled', async () => {
    await install('positive_feedback');
    await expect(node('assign').getByTestId('setup-badge')).toBeVisible();
    await expect(page.getByTestId('setup-list')).toBeVisible();
    await expect(page.locator('#automation-active')).toBeDisabled();
});
