import { expect, test } from '@playwright/test';
import { PLATFORM, waitForMail, workspaceUrl } from './platform';

/**
 * Self sign-up end to end: the form on the platform address opens the
 * company at its own sub-domain, the verification link arrives by e-mail
 * (read from the stack's Mailpit), and the new owner signs in on the
 * workspace's address.
 */

const RUN = Date.now().toString(36);
const PERSON = {
    name: 'Sam Signup',
    email: `sam+${RUN}@example.com`,
    password: 'Sunny-meadow-lantern-58', // gitleaks:allow (test fixture)
};
const COMPANY = { name: `Sunny ${RUN}`, slug: `sunny-${RUN}` };

test('sign-up, e-mail verification and the first sign-in on the new address', async ({
    page,
}) => {
    await page.goto(PLATFORM.platformUrl + '/signup');
    await expect(
        page.getByRole('heading', { name: 'Start with Messora' }),
    ).toBeVisible();
    await page.getByLabel('Full name').fill(PERSON.name);
    await page.getByLabel('Work e-mail').fill(PERSON.email);
    await page.getByLabel('Password', { exact: true }).fill(PERSON.password);
    await page.getByLabel('Company name').fill(COMPANY.name);
    await expect(page.getByLabel('Panel address')).toHaveValue(COMPANY.slug);
    await expect(page.getByText('Available.')).toBeVisible();
    await page.getByRole('checkbox').click();
    await page.getByRole('button', { name: 'Create account' }).click();
    await expect(
        page.getByRole('heading', { name: 'Check your e-mail' }),
    ).toBeVisible();

    // The same address is taken now.
    await page.goto(PLATFORM.platformUrl + '/signup');
    await page.getByLabel('Company name').fill(COMPANY.name);
    await expect(
        page.getByText('This address is already in use.'),
    ).toBeVisible();

    // The workspace answers at its sub-domain, but not before the address
    // is verified.
    await page.goto(workspaceUrl(COMPANY.slug) + '/login');
    await page.getByLabel('E-mail').fill(PERSON.email);
    await page.getByLabel('Password', { exact: true }).fill(PERSON.password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(
        page.getByText('Confirm your e-mail address first'),
    ).toBeVisible();

    const text = await waitForMail(PERSON.email, /Confirm your e-mail/);
    const link = /https?:\/\/\S+\/verify-email\?token=msv_[A-Za-z0-9_-]+/.exec(
        text,
    )?.[0];
    expect(link, 'verification link in the e-mail').toBeTruthy();
    expect(new URL(link!).host).toBe(new URL(workspaceUrl(COMPANY.slug)).host);
    await page.goto(link!);
    await expect(page.getByText('Your e-mail is verified')).toBeVisible();
    await page.getByRole('link', { name: 'Continue to sign in' }).click();
    await expect(page).toHaveURL(/\/login\?verified=1$/);
    await expect(
        page.getByText('Your e-mail address is verified.'),
    ).toBeVisible();

    await page.getByLabel('E-mail').fill(PERSON.email);
    await page.getByLabel('Password', { exact: true }).fill(PERSON.password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(
        page.getByRole('heading', { name: 'Welcome, Sam' }),
    ).toBeVisible();
    expect(new URL(page.url()).host).toBe(
        new URL(workspaceUrl(COMPANY.slug)).host,
    );

    // A second verification attempt with the used link is refused.
    await page.goto(link!);
    await expect(page.getByText('Already verified')).toBeVisible();
    // The welcome e-mail followed the verification.
    await waitForMail(PERSON.email, /Welcome to Messora|hoş geldiniz/i);
});
