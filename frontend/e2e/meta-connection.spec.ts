import { execFileSync } from 'node:child_process';
import { expect, test, type Page } from '@playwright/test';

/**
 * Settings → WhatsApp / Meta (phase 17 WP5): the company's own Meta app.
 * Needs identity.spec.ts to have run first (the user below reuses its
 * password hash).
 *
 * The E2E stack has no Meta mock, so the connection test against Graph is
 * covered by the integration tests (internal/app/meta_connection_
 * integration_test.go). Here: the page renders the guided steps, the
 * callback URL and verify token copy, the Tech Provider card is disabled,
 * and the copied address answers Meta's verification through the edge
 * proxy.
 */

const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';
const USER = { email: 'mira@example.com', password: 'Another-strong-pass-73' };
const PERMISSIONS = ['channels.read', 'channels.manage'];

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
        page.getByRole('heading', { name: 'Welcome, Mira' }),
    ).toBeVisible();
}

test.describe.configure({ mode: 'serial' });

test.beforeAll(() => {
    const hash = sql(
        `SELECT password_hash FROM users WHERE email = 'ari@example.com'`,
    );
    expect(
        hash,
        'identity.spec.ts must run first (it creates ari@example.com)',
    ).toMatch(/^\$argon2id\$/);
    sql(`
        WITH r AS (INSERT INTO roles (key, name, rank) VALUES ('e2e_meta', 'Meta (E2E)', 25)
                   ON CONFLICT (key) DO UPDATE SET name = EXCLUDED.name RETURNING id),
             p AS (INSERT INTO role_permissions (role_id, permission_key)
                   SELECT r.id, k FROM r, unnest(ARRAY['${PERMISSIONS.join("','")}']) k ON CONFLICT DO NOTHING),
             u AS (INSERT INTO users (email, name, password_hash)
                   SELECT '${USER.email}', 'Mira Meta', password_hash FROM users WHERE email = 'ari@example.com'
                   ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name RETURNING id)
        INSERT INTO tenant_memberships (user_id, role_id) SELECT u.id, r.id FROM u, r ON CONFLICT DO NOTHING;`);
});

test('the Meta connection page guides the set-up and its address answers Meta', async ({
    page,
    context,
    browserName,
    request,
}) => {
    const errors: string[] = [];
    page.on('pageerror', (e) => errors.push(e.message));
    await signIn(page);
    await page.goto('/settings/meta');
    await expect(
        page.getByRole('link', { name: 'WhatsApp / Meta' }),
    ).toHaveAttribute('aria-current', 'page');
    await expect(
        page.getByRole('heading', { name: 'WhatsApp / Meta connection' }),
    ).toBeVisible();

    // Method: the own app is chosen, the Tech Provider is not offered yet.
    await expect(page.getByTestId('method-own-app')).toHaveAttribute(
        'data-state',
        'checked',
    );
    await expect(page.getByTestId('method-tech-provider')).toBeDisabled();
    await expect(page.getByTestId('method-tech-provider')).toContainText(
        'Coming soon',
    );

    // The steps, the fields to subscribe and the workspace's own values.
    const steps = page.getByTestId('meta-steps');
    await expect(steps).toContainText('Create a Meta app');
    await expect(steps).toContainText('message_template_status_update');
    const callback = await page
        .getByLabel('Callback URL', { exact: true })
        .inputValue();
    const verify = await page
        .getByLabel('Verify token', { exact: true })
        .inputValue();
    expect(callback).toMatch(/\/meta\/[a-z2-7]{52}$/);
    expect(verify).toMatch(/^[a-z2-7]{52}$/);
    // Accounts seeded by other specs have no App secret (or are disabled):
    // signatures cannot be verified.
    await expect(page.getByTestId('signature-warning')).toBeVisible();

    // The connect form asks for the app's credentials.
    await page
        .getByTestId('meta-steps')
        .getByRole('button', { name: /^Connect (another )?account$/ })
        .click();
    const dialog = page.getByRole('dialog');
    await expect(dialog.getByLabel('App ID', { exact: true })).toHaveAttribute(
        'required',
        '',
    );
    await expect(
        dialog.getByLabel('App secret', { exact: true }),
    ).toHaveAttribute('required', '');
    await page.keyboard.press('Escape');

    // Copy buttons put the values on the clipboard (Chromium grants it).
    if (browserName === 'chromium') {
        await context.grantPermissions(['clipboard-read', 'clipboard-write']);
        await page.getByRole('button', { name: 'Copy callback url' }).click();
        await expect(page.getByText('Callback URL copied')).toBeVisible();
        expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
            callback,
        );
        await page.getByRole('button', { name: 'Copy verify token' }).click();
        expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
            verify,
        );
    }

    // Meta's verification of the copied address, through the edge proxy:
    // the right verify token gets the challenge back, a wrong one 403.
    const path = new URL(callback).pathname;
    const keyed = path.startsWith('/webhooks/') ? path : `/webhooks${path}`;
    const ok = await request.get(
        `${keyed}?hub.mode=subscribe&hub.verify_token=${verify}&hub.challenge=4711`,
    );
    expect(ok.status()).toBe(200);
    expect(await ok.text()).toBe('4711');
    const wrong = await request.get(
        `${keyed}?hub.mode=subscribe&hub.verify_token=nope&hub.challenge=1`,
    );
    expect(wrong.status()).toBe(403);
    const unknown = await request.get(
        `/webhooks/meta/${'a'.repeat(52)}?hub.mode=subscribe&hub.verify_token=${verify}&hub.challenge=1`,
    );
    expect(unknown.status()).toBe(404);

    // A new address: the old one keeps working during the grace period.
    await page.getByRole('button', { name: 'New address' }).click();
    await page.getByRole('button', { name: 'Create new address' }).click();
    await expect(page.getByText('New webhook address created')).toBeVisible();
    await expect(
        page.getByLabel('Callback URL', { exact: true }),
    ).not.toHaveValue(callback);
    await expect(page.getByText(/previous address still works/)).toBeVisible();
    const stillOk = await request.get(
        `${keyed}?hub.mode=subscribe&hub.verify_token=${verify}&hub.challenge=5`,
    );
    expect(stillOk.status()).toBe(200);

    expect(errors, 'browser errors').toEqual([]);
});
