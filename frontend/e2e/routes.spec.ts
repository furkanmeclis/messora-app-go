import { execFileSync } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { expect, test, type BrowserContext, type Page } from '@playwright/test';
import { totp } from './totp';

/**
 * Smoke test: an admin (every permission) opens every panel route and each
 * renders its heading without an error boundary or a browser error.
 *
 * Runs after identity.spec.ts. Records for the detail routes are written
 * straight into the database; the admin is invited the same way and enrols
 * two-factor authentication like any admin must.
 */

const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';
const RUN = Date.now().toString(36);
const ADMIN = {
    name: 'Rhea Routes',
    email: `rhea+${RUN}@example.com`,
    password: 'Every-route-renders-27',
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
const ids: Record<string, string> = {};

test.beforeAll(async ({ browser }) => {
    const token = `msi_${randomBytes(32).toString('base64url')}`;
    sql(`INSERT INTO user_invitations (email, role_id, token_hash, expires_at)
         SELECT '${ADMIN.email}', id, sha256(convert_to('${token}', 'UTF8')), now() + interval '1 hour'
         FROM roles WHERE key = 'admin';`);

    // One record per detail route.
    const digits = String(Date.now() % 1_000_000).padStart(6, '0');
    ids.contact = sql(`INSERT INTO contacts (phone_e164, name, source)
        VALUES ('+9055${digits}9', 'Route Contact', 'manual') RETURNING id;`);
    ids.segment = sql(`INSERT INTO segments (name, definition)
        VALUES ('Routes ${RUN}', '{"field":"name","operator":"contains","value":"Route"}') RETURNING id;`);
    ids.group = sql(
        `INSERT INTO contact_groups (name) VALUES ('Routes ${RUN}') RETURNING id;`,
    );
    ids.tag = sql(
        `INSERT INTO tags (name) VALUES ('routes-${RUN}') RETURNING id;`,
    );
    ids.automation =
        sql(`INSERT INTO automations (name, trigger_type, trigger, steps, program)
        VALUES ('Routes ${RUN}', 'new_contact', '{}',
                '[{"type":"add_tag","tag":"routes"}]',
                '[{"op":"action","step":{"type":"add_tag","tag":"routes"}}]') RETURNING id;`);
    ids.import =
        sql(`INSERT INTO contact_imports (filename, storage_key, size_bytes, headers, preview, total_rows)
        VALUES ('routes.csv', 'imports/uploads/routes-${RUN}.csv', 10, '["phone"]', '[["+905550000000"]]', 1) RETURNING id;`);
    const account =
        sql(`INSERT INTO whatsapp_accounts (name, waba_id, access_token_enc, status)
        VALUES ('Routes ${RUN}', '9${digits}1', '\\x00', 'disabled') RETURNING id;`);
    const number =
        sql(`INSERT INTO phone_numbers (account_id, phone_number_id, display_phone_number, verified_name)
        VALUES ('${account}', '9${digits}2', '+1 555 0100', 'Routes') RETURNING id;`);
    ids.campaign =
        sql(`INSERT INTO campaigns (name, phone_number_id, template_name, template_language, audience)
        VALUES ('Routes ${RUN}', '${number}', 'hello_world', 'en_US', '{"all":true}') RETURNING id;`);

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
    // Admins must use two-factor authentication: joining lands straight on
    // enrolment, rendered without a reload (regression: a chained redirect
    // left the client refetching an empty page in a loop).
    let refetches = 0;
    page.on('request', (r) => {
        if (r.url().includes('_rsc')) refetches++;
    });
    await expect(page).toHaveURL(/\/account\/security\?enroll=1/);
    await expect(
        page.getByText('Two-factor authentication is required'),
    ).toBeVisible();
    await page.waitForTimeout(1_500);
    expect(refetches, 'the enrolment page keeps refetching').toBeLessThan(10);
    await page.getByRole('button', { name: 'Set up now' }).click();
    const secret = (await page.locator('code.break-all').innerText()).trim();
    await page.getByLabel('Code from the app').fill(totp(secret));
    await page.getByRole('button', { name: 'Turn on' }).click();
    await expect(page.locator('pre span')).toHaveCount(10);
    await page.getByRole('button', { name: 'I saved them' }).click();
});

test.afterAll(async () => {
    // Leave no half-built campaign or automation for later specs.
    sql(`DELETE FROM campaigns WHERE name = 'Routes ${RUN}';
         DELETE FROM automations WHERE name = 'Routes ${RUN}';`);
    await context?.close();
});

test.afterEach(() => {
    expect(errors, 'browser errors').toEqual([]);
});

const routes: [string, () => string][] = [
    ['/', () => '/'],
    ['/inbox', () => '/inbox'],
    ['/contacts', () => '/contacts'],
    ['/contacts/[id]', () => `/contacts/${ids.contact}`],
    ['/contacts/groups', () => '/contacts/groups'],
    ['/contacts/groups/[id]', () => `/contacts/groups/${ids.group}`],
    ['/contacts/tags', () => '/contacts/tags'],
    ['/contacts/tags/[id]', () => `/contacts/tags/${ids.tag}`],
    ['/contacts/segments', () => '/contacts/segments'],
    ['/contacts/segments/new', () => '/contacts/segments/new'],
    ['/contacts/segments/[id]', () => `/contacts/segments/${ids.segment}`],
    ['/contacts/import', () => '/contacts/import'],
    ['/contacts/import/[id]', () => `/contacts/import/${ids.import}`],
    ['/campaigns', () => '/campaigns'],
    ['/campaigns/new', () => '/campaigns/new'],
    ['/campaigns/[id]', () => `/campaigns/${ids.campaign}`],
    ['/campaigns/[id]/edit', () => `/campaigns/${ids.campaign}/edit`],
    ['/templates', () => '/templates'],
    ['/channels', () => '/channels'],
    ['/automations', () => '/automations'],
    ['/automations/new', () => '/automations/new'],
    ['/automations/presets', () => '/automations/presets'],
    ['/automations/[id]', () => `/automations/${ids.automation}`],
    ['/automations/[id]/runs', () => `/automations/${ids.automation}/runs`],
    ['/users', () => '/users'],
    ['/api-keys', () => '/api-keys'],
    ['/audit', () => '/audit'],
    ['/system/failed-jobs', () => '/system/failed-jobs'],
    ['/settings', () => '/settings'],
    ['/settings/email', () => '/settings/email'],
    ['/settings/consent', () => '/settings/consent'],
    ['/settings/security', () => '/settings/security'],
    ['/settings/webhooks', () => '/settings/webhooks'],
    ['/settings/ai', () => '/settings/ai'],
    ['/settings/ai/providers', () => '/settings/ai/providers'],
    ['/settings/ai/models', () => '/settings/ai/models'],
    ['/ai-profiles', () => '/ai-profiles'],
    ['/ai-profiles/new', () => '/ai-profiles/new'],
    ['/account/profile', () => '/account/profile'],
    ['/account/security', () => '/account/security'],
];

for (const [name, path] of routes) {
    test(`renders ${name}`, async () => {
        const res = await page.goto(path());
        expect(res?.status(), `${name} status`).toBeLessThan(400);
        await expect(page.locator('main h1').first()).toBeVisible();
        await expect(page.getByText('Something went wrong')).toHaveCount(0);
        // Still the signed-in panel, not bounced to sign-in or setup.
        expect(page.url()).not.toMatch(/\/(login|setup)(\?|$)/);
    });
}

test('failed jobs page is reachable and lists the dead letter', async () => {
    await page.goto('/');
    await page
        .getByRole('link', { name: 'Failed jobs', exact: true })
        .first()
        .click();
    await expect(page).toHaveURL(/\/system\/failed-jobs$/);
    await expect(page.locator('main h1').first()).toHaveText('Failed jobs');
    // Either the empty state or rows; a fresh stack has no failed jobs.
    await expect(
        page
            .getByText('No failed jobs')
            .or(page.getByTestId('failed-job-row').first()),
    ).toBeVisible();
});

test('right-click opens the panel menu; shift keeps the browser menu', async () => {
    await page.goto('/contacts');
    await page.locator('main h1').first().click({ button: 'right' });
    const menu = page.getByRole('menu');
    await expect(
        menu.getByRole('menuitem', { name: 'Search pages and actions' }),
    ).toBeVisible();
    await expect(menu.getByRole('menuitem', { name: 'Back' })).toBeVisible();
    await page.keyboard.press('Escape');
    await expect(menu).toHaveCount(0);
    // A contact row offers its own actions first.
    const row = page
        .getByTestId('contact-row')
        .filter({ visible: true })
        .first();
    if (await row.count()) {
        await row.click({ button: 'right' });
        await expect(
            menu.getByRole('menuitem', { name: 'Copy phone number' }),
        ).toBeVisible();
        await page.keyboard.press('Escape');
    }
});
