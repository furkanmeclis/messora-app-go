import { expect, test, type Page } from '@playwright/test';
import { PLATFORM, platformSecret, workspaceUrl } from './platform';
import { totp } from './totp';

/**
 * The platform admin panel on its own host (admin.<base>): a company is
 * opened, its panel is visited through read-only support access (with the
 * banner), and a suspension closes every address of it until it is
 * lifted. Runs after identity.spec.ts, which enrols the admin's second
 * factor.
 */

const RUN = Date.now().toString(36);
const COMPANY = { name: `Support Check ${RUN}`, slug: `sup-${RUN}` };

test.describe.configure({ mode: 'serial' });

async function signIn(page: Page) {
    await page.goto(PLATFORM.adminUrl + '/login');
    await page.getByLabel('E-mail').fill(PLATFORM.email);
    await page.getByLabel('Password', { exact: true }).fill(PLATFORM.password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(page).toHaveURL(/\/login\/verify$/);
    // A code from the next step: the enrolment used the current one.
    await page
        .getByLabel('Digit 1 of 6')
        .fill(totp(platformSecret().secret, Date.now() + 30_000));
    await page.getByRole('button', { name: 'Verify' }).click();
    await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible();
}

test('the admin panel is only on the admin host', async ({ page }) => {
    // The internal route segment is never reachable directly.
    const res = await page.goto('/platform-admin/tenants');
    expect(res?.status()).toBe(404);
    // A workspace session does not open the admin panel either.
    await page.goto(PLATFORM.adminUrl + '/tenants');
    await expect(page).toHaveURL(/\/login\?next=%2Ftenants$/);
});

test('a company is opened, visited through support access, suspended and restored', async ({
    page,
    context,
}) => {
    await signIn(page);

    await page.goto(PLATFORM.adminUrl + '/tenants');
    await expect(page.getByTestId('tenant-row').first()).toBeVisible();
    await page.getByRole('button', { name: 'New company' }).click();
    const dialog = page.getByRole('dialog');
    await dialog.getByLabel('Company name').fill(COMPANY.name);
    await dialog.getByLabel('Address').fill(COMPANY.slug);
    await dialog.getByRole('button', { name: 'Create' }).click();
    await dialog.getByRole('link', { name: COMPANY.name }).click();
    await expect(
        page.getByRole('heading', { name: COMPANY.name }),
    ).toBeVisible();
    await expect(
        page.getByText(`${COMPANY.slug}.${PLATFORM.baseDomain}`).first(),
    ).toBeVisible();

    // Read-only support access opens the company's panel in a new tab.
    await page
        .getByLabel('Reason (shown to the company)')
        .fill('E2E support check');
    const [support] = await Promise.all([
        context.waitForEvent('page'),
        page.getByRole('button', { name: 'Open panel' }).click(),
    ]);
    await support.waitForURL(new RegExp(`^${workspaceUrl(COMPANY.slug)}/$`));
    const banner = support.getByTestId('support-banner');
    await expect(banner).toBeVisible();
    await expect(banner).toContainText('Read-only');
    await expect(banner).toContainText('E2E support check');
    await support.close();

    await page.goto(PLATFORM.adminUrl + '/support');
    const row = page.getByTestId('support-row').filter({
        hasText: COMPANY.name,
    });
    await expect(row.getByText('Active')).toBeVisible();
    await row.getByRole('button', { name: 'End now' }).click();
    await expect(row.getByText('Active')).toHaveCount(0);

    // Suspension: every address of the company says so.
    await page.goto(PLATFORM.adminUrl + '/tenants?q=' + COMPANY.slug);
    await page.getByRole('link', { name: COMPANY.name }).click();
    await page.getByRole('button', { name: 'Suspend' }).click();
    await page.getByRole('dialog').getByLabel('Reason').fill('Unpaid invoice');
    await page
        .getByRole('dialog')
        .getByRole('button', { name: 'Suspend' })
        .click();
    await expect(page.getByTestId('suspended-reason')).toContainText(
        'Unpaid invoice',
    );
    const visitor = await context.newPage();
    // The API caches host resolutions for 15 s on other replicas; this
    // one forgot them at once.
    await visitor.goto(workspaceUrl(COMPANY.slug) + '/login');
    await expect(
        // The company was opened in Turkish (the dialog's default).
        visitor.getByRole('heading', {
            name: /This workspace is suspended|Bu çalışma alanı askıya alındı/,
        }),
    ).toBeVisible();

    await page.getByRole('button', { name: 'Lift suspension' }).click();
    await expect(page.getByTestId('suspended-reason')).toHaveCount(0);
    await visitor.reload();
    await expect(
        visitor.getByRole('heading', { name: /^Sign in to|paneline giriş$/ }),
    ).toBeVisible();
    await visitor.close();

    // Every step is in the platform audit log.
    await page.goto(PLATFORM.adminUrl + '/audit');
    for (const action of [
        'tenant.create',
        'support.grant',
        'support.end',
        'tenant.suspend',
        'tenant.unsuspend',
    ]) {
        await expect(
            page
                .getByTestId('platform-audit-row')
                .filter({ hasText: action })
                .first(),
        ).toBeVisible();
    }
});
