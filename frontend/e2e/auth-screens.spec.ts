import { expect, test, type Page } from '@playwright/test';

/**
 * The sign-in screens without an account: inline validation, the password
 * toggle and meter, the language switch, and the states for expired or
 * broken links. Sign-up and e-mail verification call endpoints that phase
 * 17 WP2 adds; until then they show their "coming soon" states, and these
 * tests accept either.
 */

function watchErrors(page: Page) {
    const errors: string[] = [];
    page.on('pageerror', (e) => errors.push(e.message));
    return () => expect(errors, 'browser errors').toEqual([]);
}

test('sign-in checks the fields before sending them', async ({ page }) => {
    const noErrors = watchErrors(page);
    await page.goto('/login');
    await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();
    await expect(page.getByLabel('E-mail')).toBeFocused();

    await page.getByRole('button', { name: 'Sign in' }).click();
    const summary = page.getByRole('alert').filter({
        hasText: 'Check 2 fields before continuing',
    });
    await expect(summary).toBeVisible();
    await summary.getByRole('link', { name: 'Password' }).click();
    await expect(page.getByLabel('Password', { exact: true })).toBeFocused();

    await page.getByLabel('E-mail').fill('not-an-address');
    await page.getByLabel('Password', { exact: true }).fill('secret');
    await expect(
        page.getByText('Enter a valid e-mail address, like name@company.com.'),
    ).toBeVisible();

    // The eye button shows and hides what was typed.
    const password = page.getByLabel('Password', { exact: true });
    await expect(password).toHaveAttribute('type', 'password');
    await page.getByRole('button', { name: 'Show password' }).click();
    await expect(password).toHaveAttribute('type', 'text');
    await page.getByRole('button', { name: 'Hide password' }).click();
    await expect(password).toHaveAttribute('type', 'password');

    await expect(
        page.getByRole('checkbox', { name: 'Keep me signed in' }),
    ).toBeChecked();
    noErrors();
});

test('the language switch changes the screens to Turkish and back', async ({
    page,
}) => {
    await page.goto('/login');
    await page.getByRole('button', { name: 'Türkçe' }).click();
    await expect(
        page.getByRole('heading', { name: 'Giriş yap' }),
    ).toBeVisible();
    await expect(page.getByLabel('E-posta')).toBeVisible();
    await page.getByRole('button', { name: 'English' }).click();
    await expect(page.getByRole('heading', { name: 'Sign in' })).toBeVisible();
});

test('forgot password offers the form or explains why it cannot', async ({
    page,
}) => {
    await page.goto('/forgot-password');
    await expect(
        page.getByRole('heading', { name: 'Reset your password' }),
    ).toBeVisible();
    const form = page.getByRole('button', { name: 'Send reset link' });
    if (await form.isVisible()) {
        await page.getByLabel('E-mail').fill('someone@example.com');
        await form.click();
        await expect(page.getByText('Check your inbox')).toBeVisible();
        await page.getByRole('button', { name: 'Use another address' }).click();
        await expect(page.getByLabel('E-mail')).toBeVisible();
    } else {
        await expect(
            page.getByText('E-mail is not configured on this installation.'),
        ).toBeVisible();
    }
    await page.getByRole('link', { name: 'Back to sign in' }).click();
    await expect(page).toHaveURL(/\/login$/);
});

test('an expired reset link says so and offers a new one', async ({ page }) => {
    const noErrors = watchErrors(page);
    await page.goto('/reset-password?token=msr_not-a-real-token');
    const password = page.getByLabel('New password', { exact: true });
    await password.fill('short');
    await expect(page.getByText('7 more characters')).toBeVisible();
    await password.fill('Correct-horse-staple-91');
    await expect(page.getByText('Strength: strong')).toBeVisible();
    await page.getByRole('button', { name: 'Set new password' }).click();
    await expect(page.getByText('This reset link has expired')).toBeVisible();
    await page.getByRole('link', { name: 'Request a new link' }).click();
    await expect(page).toHaveURL(/\/forgot-password$/);
    noErrors();
});

test('a broken invitation link explains what to do', async ({ page }) => {
    await page.goto('/invite?token=msi_not-a-real-token');
    await expect(
        page.getByRole('heading', { name: 'This invitation does not work' }),
    ).toBeVisible();
    await expect(
        page.getByText('Ask an administrator for a new invitation.'),
    ).toBeVisible();
});

test('sign-up previews the panel address and checks its rules', async ({
    page,
}) => {
    const noErrors = watchErrors(page);
    await page.goto('/signup');
    await expect(
        page.getByRole('heading', { name: 'Start with Messora' }),
    ).toBeVisible();

    await page.getByLabel('Company name').fill('Easy Trade A.Ş.');
    const slug = page.getByLabel('Panel address');
    await expect(slug).toHaveValue('easy-trade-a-s');
    await expect(page.getByText(/^easy-trade-a-s\./)).toBeVisible();

    await slug.fill('Admin');
    await expect(slug).toHaveValue('admin');
    await slug.blur();
    await expect(
        page.getByText('This address is reserved. Try another.').first(),
    ).toBeVisible();

    await page.getByRole('button', { name: 'Create account' }).click();
    await expect(
        page.getByText(/^Check \d+ fields before continuing:/),
    ).toBeVisible();
    await expect(page.getByText('Accept the terms to continue.')).toBeVisible();

    await page.getByLabel('Full name').fill('Ayşe Yılmaz');
    await page.getByLabel('Work e-mail').fill('ayse@example.com');
    await page
        .getByLabel('Password', { exact: true })
        .fill('Mavi-deniz-kahve-42');
    await slug.fill('easytrade-e2e');
    await page.getByRole('checkbox').click();
    await page.getByRole('button', { name: 'Create account' }).click();
    // Until the API has sign-up (phase 17 WP2) the form says it opens soon.
    await expect(
        page
            .getByText('Sign-ups open soon')
            .or(page.getByRole('heading', { name: 'Check your e-mail' })),
    ).toBeVisible();
    noErrors();
});

test('a verification link without the API endpoint degrades gently', async ({
    page,
}) => {
    await page.goto('/verify-email?token=msv_not-a-real-token');
    await expect(
        page
            .getByText('Verification is not open yet')
            .or(page.getByText('This link does not work')),
    ).toBeVisible();
});

test('the code screen needs a pending sign-in', async ({ page }) => {
    await page.goto('/login/verify');
    await expect(page).toHaveURL(/\/login$/);
});
