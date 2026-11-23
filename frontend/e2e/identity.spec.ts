import { expect, test, type Page } from '@playwright/test';
import { totp } from './totp';

/**
 * The whole identity journey on a fresh installation, in order:
 * setup → mandatory 2FA enrolment → invitation → invitee joins with a
 * restricted role → owner signs in with a second factor → API key → audit.
 * Any uncaught browser error fails the test.
 */

const OWNER = {
    org: 'Acme E2E',
    name: 'Olivia Owner',
    email: 'olivia@example.com',
    password: 'Correct-horse-staple-91',
};
const AGENT = {
    name: 'Ari Agent',
    email: 'ari@example.com',
    password: 'Another-strong-pass-73',
};

const state: { secret?: string; recovery?: string[]; inviteUrl?: string } = {};

function watchErrors(page: Page) {
    const errors: string[] = [];
    page.on('pageerror', (e) => errors.push(e.message));
    page.on('console', (m) => {
        if (m.type() === 'error') errors.push(m.text());
    });
    return () => expect(errors, 'browser errors').toEqual([]);
}

/** A code from the next 30-second step, so it is never one already used. */
function freshCode(secret: string) {
    return totp(secret, Date.now() + 30_000);
}

test.describe.configure({ mode: 'serial' });

test('owner sets up the installation and enrols two-factor authentication', async ({ page }) => {
    const noErrors = watchErrors(page);
    const setupUrl = process.env.E2E_SETUP_URL;
    expect(setupUrl, 'E2E_SETUP_URL must be set').toBeTruthy();

    await page.goto(setupUrl!);
    await expect(page.getByText('Set up Messora')).toBeVisible();
    await page.getByLabel('Organization name').fill(OWNER.org);
    await page.getByLabel('Your name').fill(OWNER.name);
    await page.getByLabel('E-mail').fill(OWNER.email);
    await page.getByLabel('Password').fill(OWNER.password);
    await page.getByRole('button', { name: 'Create owner account' }).click();

    // Owners must enrol 2FA before anything else.
    await expect(page).toHaveURL(/\/account\/security/);
    await expect(page.getByText('Two-factor authentication is required')).toBeVisible();
    await page.goto('/users');
    await expect(page).toHaveURL(/\/account\/security\?enroll=1/);

    await page.getByRole('button', { name: 'Set up now' }).click();
    const secret = (await page.locator('code.break-all').innerText()).trim();
    expect(secret).toMatch(/^[A-Z2-7]{16,}$/);
    await expect(page.getByAltText('QR code for your authenticator app')).toBeVisible();
    await page.getByLabel('Code from the app').fill(totp(secret));
    await page.getByRole('button', { name: 'Turn on' }).click();

    const codes = page.locator('pre span');
    await expect(codes).toHaveCount(10);
    state.recovery = await codes.allInnerTexts();
    state.secret = secret;
    await page.getByRole('button', { name: 'I saved them' }).click();

    await page.goto('/');
    await expect(page.getByRole('heading', { name: 'Welcome, Olivia' })).toBeVisible();
    await expect(page.getByText('All systems healthy')).toBeVisible();
    await expect(page.getByText(OWNER.org).first()).toBeVisible();
    noErrors();
});

test('owner invites an agent', async ({ page }) => {
    const noErrors = watchErrors(page);
    await signIn(page, OWNER.email, OWNER.password, () => state.recovery!.shift()!);

    await page.goto('/users');
    await page.getByRole('button', { name: 'Invite user' }).click();
    const dialog = page.getByRole('dialog');
    await dialog.getByLabel('E-mail').fill(AGENT.email);
    await dialog.getByRole('button', { name: 'Send invitation' }).click();
    // E-mail is not configured in the test stack, so the link is shown once.
    const link = dialog.locator('input[readonly]');
    await expect(link).toHaveValue(/\/invite\?token=msi_/);
    state.inviteUrl = await link.inputValue();
    await dialog.getByRole('button', { name: 'Done' }).click();
    await expect(page.getByText('Pending invitations')).toBeVisible();
    // The address appears in the invitation row (and may also be echoed in
    // the success toast); one visible match is enough.
    await expect(page.getByText(AGENT.email).first()).toBeVisible();
    noErrors();
});

test('invitee joins as an agent and sees only what the role allows', async ({ browser }) => {
    const context = await browser.newContext();
    const page = await context.newPage();
    const noErrors = watchErrors(page);

    const url = new URL(state.inviteUrl!);
    await page.goto(url.pathname + url.search);
    await expect(page.getByText('You were invited as Agent.')).toBeVisible();
    await page.getByLabel('Your name').fill(AGENT.name);
    await page.getByLabel('Password').fill('password1234');
    await page.getByRole('button', { name: 'Join' }).click();
    await expect(page.getByText(/too common|varied/)).toBeVisible();
    // A rejected submission keeps what the user typed (except the password).
    await expect(page.getByLabel('Your name')).toHaveValue(AGENT.name);
    await page.getByLabel('Password').fill(AGENT.password);
    await page.getByRole('button', { name: 'Join' }).click();

    await expect(page.getByRole('heading', { name: 'Welcome, Ari' })).toBeVisible();
    const nav = page.locator('[data-sidebar="sidebar"]');
    await expect(nav.getByRole('link', { name: 'Overview' })).toBeVisible();
    await expect(nav.getByRole('link', { name: 'Users' })).toHaveCount(0);
    await expect(nav.getByRole('link', { name: 'API keys' })).toHaveCount(0);
    // Direct navigation is refused too.
    await page.goto('/users');
    await expect(page).toHaveURL(/\/$/);
    noErrors();
    await context.close();
});

test('owner signs in with an authenticator code and creates an API key', async ({ page }) => {
    const noErrors = watchErrors(page);
    await signIn(page, OWNER.email, OWNER.password, () => freshCode(state.secret!));

    await page.goto('/api-keys');
    await expect(page.getByText('No API keys yet')).toBeVisible();
    await page.getByRole('button', { name: 'Create key' }).click();
    const dialog = page.getByRole('dialog');
    await dialog.getByLabel('Name').fill('CRM sync');
    await dialog.getByLabel('contacts.read').check();
    await dialog.getByRole('button', { name: 'Create key' }).click();
    await expect(dialog.locator('input[readonly]')).toHaveValue(/^msk_live_/);
    await dialog.getByRole('button', { name: 'Done' }).click();
    await expect(page.getByRole('cell', { name: /CRM sync/ })).toBeVisible();

    await page.goto('/audit');
    for (const action of ['setup.complete', 'user.mfa_enable', 'invitation.create', 'invitation.accept', 'api_key.create']) {
        await expect(page.getByRole('button', { name: action }).first()).toBeVisible();
    }
    noErrors();
});

test('wrong password and unknown e-mail look the same', async ({ page }) => {
    await page.goto('/login');
    for (const email of [OWNER.email, 'nobody@example.com']) {
        await page.getByLabel('E-mail').fill(email);
        await page.getByLabel('Password').fill('definitely-wrong');
        await page.getByRole('button', { name: 'Sign in' }).click();
        await expect(page.getByText('E-mail or password is incorrect')).toBeVisible();
    }
});

async function signIn(page: Page, email: string, password: string, secondFactor: () => string) {
    await page.goto('/login');
    await page.getByLabel('E-mail').fill(email);
    await page.getByLabel('Password').fill(password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(page).toHaveURL(/\/login\/verify/);
    const code = secondFactor();
    if (code.includes('-')) {
        await page.getByRole('button', { name: 'Use a recovery code instead' }).click();
        await page.getByLabel('Recovery code').fill(code);
    } else {
        await page.getByLabel('Authenticator code').fill(code);
    }
    await page.getByRole('button', { name: 'Verify' }).click();
    await expect(page).not.toHaveURL(/\/login/);
}
