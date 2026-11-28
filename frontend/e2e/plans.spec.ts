import { execFileSync } from 'node:child_process';
import { expect, test, type BrowserContext, type Page } from '@playwright/test';
import { PLATFORM, platformSecret, workspaceUrl } from './platform';
import { totp } from './totp';

/**
 * Phase 17 WP8: plans and entitlements. The workspace's Plan & usage page
 * shows its plan, usage and the plan comparison; the e-mail and AI pages
 * hide the company's own SMTP server and AI keys unless the plan is
 * Enterprise (the second company "beta" is moved between plans in the
 * platform admin panel and visited through support access); a plan is
 * edited in the admin panel; the Meta rate card is read-only and public as
 * JSON. Needs identity.spec.ts first (users, the admin's second factor).
 */

const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';
const USER = { email: 'pia@example.com', password: 'Another-strong-pass-73' };
const PERMISSIONS = ['settings.read', 'settings.update', 'ai.read'];

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
            'PGOPTIONS="-c app.tenant_id=00000000-0000-0000-0000-000000000001" psql -v ON_ERROR_STOP=1 -qtA -U "$POSTGRES_USER" -d "$POSTGRES_DB"',
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
        page.getByRole('heading', { name: 'Welcome, Pia' }),
    ).toBeVisible();
}

/** Signs the platform admin in (the current TOTP code, else the next one). */
async function adminSignIn(page: Page) {
    await page.goto(PLATFORM.adminUrl + '/login');
    await page.getByLabel('E-mail').fill(PLATFORM.email);
    await page.getByLabel('Password', { exact: true }).fill(PLATFORM.password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(page).toHaveURL(/\/login\/verify$/);
    for (let attempt = 0; ; attempt++) {
        await page
            .getByLabel('Digit 1 of 6')
            .fill(totp(platformSecret().secret, Date.now()));
        await page.getByRole('button', { name: 'Verify' }).click();
        const ok = await page
            .getByRole('heading', { name: 'Overview' })
            .waitFor({ timeout: 5_000 })
            .then(() => true)
            .catch(() => false);
        if (ok) break;
        if (attempt > 0) throw new Error('platform admin sign-in failed');
        await page.waitForTimeout(31_000 - (Date.now() % 30_000));
    }
}

/** Opens beta's tenant page in the admin panel. */
async function openBeta(page: Page) {
    await page.goto(PLATFORM.adminUrl + '/tenants?q=beta');
    await page.getByRole('link', { name: 'Beta E2E' }).click();
    await expect(page.getByTestId('tenant-plan')).toBeVisible();
}

/** Assigns a plan to the open company (English admin panel). */
async function assignPlan(page: Page, name: string) {
    await page.getByTestId('tenant-plan-select').click();
    await page.getByRole('option', { name, exact: true }).click();
    await page.getByTestId('tenant-plan-save').click();
    await expect(page.getByText('Plan saved.')).toBeVisible();
    await expect(page.getByTestId('tenant-plan-key')).toHaveText(name);
}

/** Opens beta's panel through read-only support access. */
async function supportPanel(page: Page, ctx: BrowserContext): Promise<Page> {
    await page
        .getByLabel('Reason (shown to the company)')
        .fill('E2E plan check');
    const [support] = await Promise.all([
        ctx.waitForEvent('page'),
        page.getByRole('button', { name: 'Open panel' }).click(),
    ]);
    await support.waitForURL(new RegExp(`^${workspaceUrl('beta')}/$`));
    await expect(support.getByTestId('support-banner')).toBeVisible();
    return support;
}

test.describe.configure({ mode: 'serial' });

test.beforeAll(() => {
    const hash = sql(
        `SELECT password_hash FROM users WHERE email = 'ari@example.com'`,
    );
    expect(hash, 'identity.spec.ts must run first').toMatch(/^\$argon2id\$/);
    sql(`
        WITH r AS (INSERT INTO roles (key, name, rank) VALUES ('e2e_planner', 'Planner (E2E)', 25)
                   ON CONFLICT (key) DO UPDATE SET name = EXCLUDED.name RETURNING id),
             p AS (INSERT INTO role_permissions (role_id, permission_key)
                   SELECT r.id, k FROM r, unnest(ARRAY['${PERMISSIONS.join("','")}']) k ON CONFLICT DO NOTHING),
             u AS (INSERT INTO users (email, name, password_hash, email_verified_at)
                   SELECT '${USER.email}', 'Pia Planner', password_hash, now() FROM users WHERE email = 'ari@example.com'
                   ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name RETURNING id)
        INSERT INTO tenant_memberships (user_id, role_id) SELECT u.id, r.id FROM u, r ON CONFLICT DO NOTHING;`);
});

test('the plan page shows the plan, its usage and the comparison', async ({
    page,
}) => {
    await signIn(page);
    await page.goto('/settings/plan');
    await expect(page.getByTestId('plan-page')).toBeVisible();
    await expect(page.getByTestId('plan-name')).not.toBeEmpty();
    for (const r of [
        'users',
        'phone_numbers',
        'ai_agents',
        'automation_flows',
        'storage_bytes',
    ]) {
        await expect(page.getByTestId(`usage-${r}`)).toBeVisible();
    }
    // At least one member: the user signed in.
    await expect(page.getByTestId('usage-users')).toContainText(/[1-9]/);
    await expect(page.getByTestId('plan-credits')).toBeVisible();
    await expect(page.getByTestId('plan-features')).toBeVisible();
    const compare = page.getByTestId('plan-compare');
    for (const name of ['Starter', 'Growth', 'Pro', 'Enterprise']) {
        await expect(
            compare.getByRole('columnheader', { name: new RegExp(name) }),
        ).toBeVisible();
    }
    await expect(compare).toContainText('+VAT');
    await expect(compare.getByText('Your plan')).toBeVisible();
});

test('the Meta rate card is read-only and published as JSON', async ({
    page,
    playwright,
    baseURL,
}) => {
    await signIn(page);
    await page.goto('/settings/meta-rates');
    await expect(page.getByTestId('meta-rates-public')).toContainText(
        '/api/v1/meta-rates',
    );
    // No import on the workspace side.
    await expect(page.locator('input[type="file"]')).toHaveCount(0);
    await expect(page.getByText(/import it here/i)).toHaveCount(0);
    // Any host, no session.
    const anon = await playwright.request.newContext({ baseURL });
    const res = await anon.get('/api/v1/meta-rates');
    expect(res.status()).toBe(200);
    expect(res.headers()['content-type']).toContain('application/json');
    const body = (await res.json()) as { data: unknown[] };
    expect(Array.isArray(body.data)).toBe(true);
    await anon.dispose();
});

test("own SMTP and AI keys appear only on beta's Enterprise plan", async ({
    browser,
}) => {
    test.skip(!process.env.E2E_ADMIN_URL, 'needs the admin host');
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    await adminSignIn(page);

    // Growth: Messora's sender and keys only.
    await openBeta(page);
    await assignPlan(page, 'Growth');
    let panel = await supportPanel(page, ctx);
    await panel.goto(workspaceUrl('beta') + '/settings/email');
    await expect(panel.getByTestId('email-mode')).toHaveAttribute(
        'data-mode',
        'platform',
    );
    await expect(panel.getByTestId('smtp-locked')).toBeVisible();
    await expect(panel.locator('#smtp-host')).toHaveCount(0);
    await panel.goto(workspaceUrl('beta') + '/settings/ai/providers');
    await expect(panel.getByTestId('platform-ai-providers')).toBeVisible();
    await expect(panel.getByTestId('byo-locked')).toBeVisible();
    await panel.goto(workspaceUrl('beta') + '/settings/plan');
    await expect(panel.getByTestId('plan-name')).toHaveAttribute(
        'data-plan',
        'growth',
    );
    await panel.close();

    // Enterprise: the company's own server and keys.
    await page.reload();
    await assignPlan(page, 'Enterprise');
    panel = await supportPanel(page, ctx);
    await panel.goto(workspaceUrl('beta') + '/settings/email');
    await expect(panel.getByTestId('smtp-locked')).toHaveCount(0);
    await expect(panel.locator('#smtp-host')).toBeVisible();
    await panel.goto(workspaceUrl('beta') + '/settings/ai/providers');
    await expect(panel.getByTestId('byo-locked')).toHaveCount(0);
    await expect(panel.getByTestId('own-ai-keys')).toBeVisible();
    await panel.close();

    // Back to Growth, as other specs expect nothing of beta's plan.
    await page.reload();
    await assignPlan(page, 'Growth');
    await ctx.close();
});

test('the platform admin edits a plan', async ({ browser }) => {
    test.skip(!process.env.E2E_ADMIN_URL, 'needs the admin host');
    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    await adminSignIn(page);
    await page.goto(PLATFORM.adminUrl + '/plans');
    const row = page.locator('[data-testid="plan-row"][data-plan="starter"]');
    await expect(row.getByTestId('plan-users')).toHaveText('2');

    const edit = async (users: string) => {
        await row.getByTestId('plan-edit').click();
        const dialog = page.getByRole('dialog');
        const field = dialog.getByLabel('Users', { exact: true });
        await field.fill(users);
        await dialog.getByTestId('plan-save').click();
        await expect(page.getByText('Plan saved.')).toBeVisible();
        await expect(row.getByTestId('plan-users')).toHaveText(users);
    };
    await edit('4');
    await edit('2');

    // The change is in the platform audit log.
    await page.goto(PLATFORM.adminUrl + '/audit');
    await expect(
        page
            .getByTestId('platform-audit-row')
            .filter({ hasText: 'plan.update' })
            .first(),
    ).toBeVisible();
    await ctx.close();
});
