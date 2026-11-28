import { execFileSync } from 'node:child_process';
import { expect, test, type Page } from '@playwright/test';
import { PLATFORM, platformSecret, workspaceUrl } from './platform';
import { totp } from './totp';

/**
 * Phase 17 WP9: subscriptions and billing by bank transfer. A company on
 * trial ("gamma", opened here as a fixture) fills its invoice details,
 * chooses a plan and gets a proforma with the payment reference and
 * Messora's IBAN; the platform admin enters the seller, records the
 * incoming transfer in the payments inbox and confirms the suggested
 * match; the company is active with an invoice record. Then the admin
 * sets the company read-only and the panel shows the banner. With
 * WP9_SCREENS set, screenshots (light and dark) go to that directory.
 * Needs identity.spec.ts first (the admin's second factor, a password).
 */

const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';
const SLUG = 'gamma';
const USER = { email: 'bea@example.com', password: 'Another-strong-pass-73' };
const IBAN = 'TR330006100519786457841326';
const SCREENS = process.env.WP9_SCREENS;

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

async function shoot(page: Page, name: string) {
    if (!SCREENS) return;
    for (const scheme of ['light', 'dark'] as const) {
        await page.emulateMedia({ colorScheme: scheme });
        await page.waitForTimeout(300);
        await page.screenshot({
            path: `${SCREENS}/${name}-${scheme}.png`,
            fullPage: true,
        });
    }
    await page.emulateMedia({ colorScheme: 'light' });
}

async function signIn(page: Page) {
    await page.goto(workspaceUrl(SLUG) + '/login');
    await page.getByLabel('E-mail').fill(USER.email);
    await page.getByLabel('Password', { exact: true }).fill(USER.password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(page).not.toHaveURL(/\/login/);
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

test.describe.configure({ mode: 'serial' });

test.beforeAll(() => {
    const hash = sql(
        `SELECT password_hash FROM users WHERE email = 'ari@example.com'`,
    );
    expect(hash, 'identity.spec.ts must run first').toMatch(/^\$argon2id\$/);
    // A company on trial (growth limits) with one member who manages
    // settings (a custom role: no mandatory second factor).
    sql(`
        INSERT INTO tenants (slug, name, subscription_status, trial_ends_at, plan_key)
        VALUES ('${SLUG}', 'Gamma E2E', 'trial', now() + interval '10 days', 'growth')
        ON CONFLICT (slug) DO NOTHING;
        INSERT INTO tenant_domains (tenant_id, hostname, kind, verified_at, is_primary)
        SELECT id, '${SLUG}.${PLATFORM.baseDomain}', 'subdomain', now(), true FROM tenants WHERE slug = '${SLUG}'
        ON CONFLICT (hostname) DO NOTHING;
        INSERT INTO roles (key, name, rank) VALUES ('e2e_biller', 'Biller (E2E)', 25)
        ON CONFLICT (key) DO UPDATE SET name = EXCLUDED.name;
        INSERT INTO role_permissions (role_id, permission_key)
        SELECT r.id, k FROM roles r, unnest(ARRAY['settings.read', 'settings.update']) k
        WHERE r.key = 'e2e_biller' ON CONFLICT DO NOTHING;
        INSERT INTO users (email, name, password_hash, email_verified_at)
        SELECT '${USER.email}', 'Bea Biller', password_hash, now() FROM users WHERE email = 'ari@example.com'
        ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name;
        INSERT INTO tenant_memberships (tenant_id, user_id, role_id)
        SELECT t.id, u.id, r.id FROM tenants t, users u, roles r
        WHERE t.slug = '${SLUG}' AND u.email = '${USER.email}' AND r.key = 'e2e_biller'
        ON CONFLICT DO NOTHING;`);
});

let reference = '';

test('the admin enters the seller and its bank account', async ({ page }) => {
    await adminSignIn(page);
    await page.goto(PLATFORM.adminUrl + '/billing-settings');
    const seller = page.getByTestId('seller-settings');
    await seller.locator('#s-company').fill('Messora Teknoloji A.Ş.');
    await seller.locator('#s-tax-office').fill('Beşiktaş');
    await seller.locator('#s-tax-number').fill('9876543210');
    await seller.locator('#s-address').fill('Levent, İstanbul');
    if ((await seller.getByTestId('bank-account').count()) === 0) {
        await seller.getByTestId('add-bank').click();
    }
    await seller.locator('#b-name-0').fill('Örnek Bank');
    await seller.locator('#b-holder-0').fill('Messora Teknoloji A.Ş.');
    await seller.locator('#b-iban-0').fill('TR33 0006 1005 1978 6457 8413 26');
    await seller.getByRole('button', { name: 'Save' }).click();
    await expect(page.getByText('Billing settings saved.')).toBeVisible();
});

test('a trial company enters its invoice details and gets a proforma', async ({
    page,
}) => {
    await signIn(page);
    await page.goto(workspaceUrl(SLUG) + '/settings/billing');
    await expect(page.getByTestId('billing-page')).toBeVisible();
    await expect(page.getByTestId('billing-status')).toHaveAttribute(
        'data-status',
        'trial',
    );
    const profile = page.getByTestId('billing-profile');
    await profile.locator('#bp-title').fill('Gamma Gıda Ltd. Şti.');
    await profile.locator('#bp-tax_office').fill('Kadıköy');
    await profile.locator('#bp-tax_number').fill('1234567890');
    await profile.locator('#bp-address').fill('Moda Cd. 1, Kadıköy');
    await profile.locator('#bp-city').fill('İstanbul');
    await profile.locator('#bp-email').fill('muhasebe@gamma.test');
    await page.getByRole('button', { name: 'Save' }).click();
    await expect(page.getByText('Invoice details saved.')).toBeVisible();

    await page
        .locator('[data-testid="billing-plan-option"][data-plan="growth"]')
        .getByTestId('choose-plan')
        .click();
    await expect(page.getByTestId('plan-change-result')).toBeVisible();
    const card = page.getByTestId('proforma-card').first();
    await expect(card).toBeVisible();
    await expect(card.getByTestId('proforma-total')).toContainText(
        /3[.,]588[.,]00/,
    );
    await expect(card.getByTestId('bank-iban')).toHaveText(
        IBAN.replace(/(.{4})(?=.)/g, '$1 '),
    );
    await expect(card.getByTestId('proforma-reference')).toHaveText(
        /^MSR-[0-9A-Z]{4}-[0-9A-Z]{4}-[0-9A-Z]$/,
    );
    reference = (await card.getAttribute('data-reference')) ?? '';
    expect(reference).toMatch(/^MSR[0-9A-Z]{9}$/);
    // The PDF downloads through the panel.
    const pdf = await page.request.get(
        `${workspaceUrl(SLUG)}/bff/billing/proformas/${await card.getAttribute('data-id')}/pdf`,
    );
    expect(pdf.ok()).toBe(true);
    expect((await pdf.body()).subarray(0, 5).toString()).toBe('%PDF-');
    await shoot(page, 'tenant-billing');
});

test('the admin records the transfer and confirms the match', async ({
    page,
}) => {
    expect(reference).not.toBe('');
    await adminSignIn(page);
    await page.goto(PLATFORM.adminUrl + '/payments');
    const form = page.getByTestId('record-transfer');
    await form.locator('#tr-amount').fill('3588.00');
    await form.locator('#tr-sender_name').fill('GAMMA GIDA');
    await form
        .locator('#tr-description')
        .fill(
            `EFT ${reference.slice(0, 3)}-${reference.slice(3, 7)}-${reference.slice(7, 11)}-${reference.slice(11)} abonelik`,
        );
    await form.getByTestId('record-transfer-submit').click();
    await expect(page.getByText('Transfer recorded.')).toBeVisible();
    const row = page.locator(
        `[data-testid="transfer"][data-reference="${reference}"]`,
    );
    await expect(row.getByTestId('suggestion')).toHaveAttribute(
        'data-suggestion',
        'reference_amount',
    );
    await shoot(page, 'admin-payments');
    await row.getByTestId('confirm-match').click();
    await expect(page.getByText('Payment confirmed.')).toBeVisible();
});

test('the company is active with an invoice record', async ({ page }) => {
    await signIn(page);
    await page.goto(workspaceUrl(SLUG) + '/settings/billing');
    await expect(page.getByTestId('billing-status')).toHaveAttribute(
        'data-status',
        'active',
    );
    await expect(page.getByTestId('billing-plan')).toHaveAttribute(
        'data-plan',
        'growth',
    );
    await expect(page.getByTestId('no-open-proformas')).toBeVisible();
    await expect(page.getByTestId('invoice-row').first()).toHaveAttribute(
        'data-status',
        'pending',
    );
    await shoot(page, 'tenant-billing-active');
});

test('a read-only company sees the banner', async ({ page, browser }) => {
    await adminSignIn(page);
    await page.goto(PLATFORM.adminUrl + '/tenants?q=' + SLUG);
    await page.getByRole('link', { name: 'Gamma E2E' }).click();
    const box = page.getByTestId('tenant-billing');
    await expect(box).toBeVisible();
    await box.getByTestId('billing-action-select').click();
    await page.getByRole('option', { name: 'Set the status' }).click();
    await box.getByTestId('billing-status-select').click();
    await page.getByRole('option', { name: 'Read-only' }).click();
    await box.locator('#ba-reason').fill('E2E read-only check');
    await box.getByTestId('billing-action-apply').click();
    await expect(page.getByText('Done.')).toBeVisible();

    const tenant = await (await browser.newContext()).newPage();
    await signIn(tenant);
    await tenant.goto(workspaceUrl(SLUG) + '/settings/billing');
    await expect(tenant.getByTestId('billing-banner')).toHaveAttribute(
        'data-kind',
        'read_only',
    );
    await expect(tenant.getByTestId('billing-read-only')).toBeVisible();
    await shoot(tenant, 'tenant-read-only');
});
