import { execFileSync } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { readFile } from 'node:fs/promises';
import { expect, test, type BrowserContext, type Page } from '@playwright/test';

/**
 * Contacts journey: add and edit a contact, group contacts, import a CSV
 * through the wizard, build a segment with a live count and export.
 *
 * Runs after identity.spec.ts (see playwright.config.ts). The owner's second
 * factor lives only in that spec, so this one signs in as a fresh manager:
 * it writes an invitation for a token it knows straight into the database
 * (exactly what the API stores: SHA-256 of the token) and accepts it in the
 * browser like any invitee.
 */

const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';
const RUN = Date.now().toString(36);
const MANAGER = {
    name: 'Mira Manager',
    email: `mira+${RUN}@example.com`,
    password: 'Plenty-of-entropy-58',
};
// Numbers unique to this run (so the spec also passes on a kept stack);
// PREFIX matches exactly the contacts this spec creates.
const PREFIX = `+90555${String(Date.now() % 10_000).padStart(4, '0')}`;
const PHONE = (i: number) => `${PREFIX}${String(i).padStart(3, '0')}`;

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
const errors: string[] = [];
let contactUrl = '';

test.beforeAll(async ({ browser }) => {
    const token = `msi_${randomBytes(32).toString('base64url')}`;
    sql(`INSERT INTO user_invitations (email, role_id, token_hash, expires_at)
         SELECT '${MANAGER.email}', id, sha256(convert_to('${token}', 'UTF8')), now() + interval '1 hour'
         FROM roles WHERE key = 'manager';`);

    context = await browser.newContext();
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
        page.getByRole('heading', { name: 'Welcome, Mira' }),
    ).toBeVisible();
});

test.afterAll(async () => {
    await context?.close();
});

test.afterEach(() => {
    expect(errors, 'browser errors').toEqual([]);
});

test('a manager adds a contact', async () => {
    await page
        .locator('[data-sidebar="sidebar"]')
        .getByRole('link', { name: 'Contacts' })
        .click();
    await expect(
        page.getByRole('heading', { name: 'Contacts', exact: true }),
    ).toBeVisible();

    await page.getByRole('button', { name: 'New contact' }).click();
    const dialog = page.getByRole('dialog');
    await dialog.getByLabel('Phone number').fill('0555 abc 12');
    await dialog.getByRole('button', { name: 'Add contact' }).click();
    // The API's field error appears under the field.
    await expect(dialog.getByRole('alert')).toContainText(
        'international format',
    );
    await dialog.getByLabel('Phone number').fill(PHONE(1));
    await dialog.getByLabel('Name', { exact: true }).fill('Ece Yılmaz');
    await dialog.getByLabel('E-mail').fill('ece@example.com');
    await dialog.getByRole('button', { name: 'Add contact' }).click();

    await expect(page).toHaveURL(/\/contacts\/[0-9a-f-]{36}$/);
    await expect(
        page.getByRole('heading', { name: 'Ece Yılmaz' }),
    ).toBeVisible();
    contactUrl = page.url();
});

test('the contact is edited, tagged, noted and opted out', async () => {
    await page.goto(contactUrl);
    await page.getByLabel('Variable 1').fill('Gold');
    const tags = page.getByLabel('Tags', { exact: true });
    await tags.fill('vip');
    await tags.press('Enter');
    await page.getByRole('button', { name: 'Add field' }).click();
    await page.getByLabel('Custom field 1 name').fill('city');
    await page.getByLabel('Custom field 1 value').fill('İzmir');
    await page.getByRole('button', { name: 'Save changes' }).click();
    await expect(page.getByText('Contact saved.')).toBeVisible();

    await page.reload();
    await expect(page.getByLabel('Variable 1')).toHaveValue('Gold');
    await expect(page.getByRole('list', { name: 'Chosen tags' })).toContainText(
        'vip',
    );
    await expect(page.getByLabel('Custom field 1 value')).toHaveValue('İzmir');

    await page.getByLabel('New note').fill('Prefers messages in the morning');
    await page.getByRole('button', { name: 'Add note' }).click();
    await expect(
        page.getByText('Prefers messages in the morning'),
    ).toBeVisible();

    // Consent is kept per category and changed by hand with a reason.
    const changeConsent = async (
        category: 'Marketing' | 'Service' | 'Both',
        status: string,
        reason?: string,
    ) => {
        await page.getByRole('button', { name: 'Change consent' }).click();
        const dialog = page.getByRole('dialog');
        await dialog.getByRole('combobox', { name: 'Category' }).click();
        await page.getByRole('option', { name: category, exact: true }).click();
        await dialog.getByRole('combobox', { name: 'New status' }).click();
        await page.getByRole('option', { name: status, exact: true }).click();
        if (reason) await dialog.getByLabel(/^Reason/).fill(reason);
        await dialog.getByRole('button', { name: 'Save', exact: true }).click();
        await expect(dialog).toHaveCount(0);
    };
    await changeConsent('Marketing', 'Opted out', 'Asked by phone');
    await expect(page.getByTestId('consent-marketing')).toHaveAttribute(
        'data-status',
        'opted_out',
    );
    await expect(page.getByTestId('consent-marketing')).toContainText(
        'Marketing: Opted out',
    );
    await expect(page.getByTestId('consent-service')).not.toHaveAttribute(
        'data-status',
        'opted_out',
    );
    await expect(
        page.getByTestId('consent-event').filter({ hasText: 'Asked by phone' }),
    ).toContainText('Opted out');

    await changeConsent('Both', 'Opted in');
    await expect(page.getByTestId('consent-marketing')).toHaveAttribute(
        'data-status',
        'opted_in',
    );
    await expect(page.getByTestId('consent-service')).toHaveAttribute(
        'data-status',
        'opted_in',
    );
});

test('contacts are listed and filtered by consent', async () => {
    const q = encodeURIComponent(PHONE(1));
    await page.goto(`/contacts?q=${q}&marketing_consent=opted_in`);
    const row = page.getByTestId('contact-row');
    await expect(row).toHaveCount(1);
    await expect(row.getByTestId('consent-marketing')).toHaveAttribute(
        'data-status',
        'opted_in',
    );
    await expect(page.getByLabel('Marketing consent')).toContainText(
        'Opted in',
    );
    await page.goto(`/contacts?q=${q}&marketing_consent=opted_out`);
    await expect(page.getByText('No matching contacts')).toBeVisible();
});

test('a CSV file is imported through the wizard', async () => {
    await page.goto('/contacts/import');
    const csv = [
        'Phone,Full name,E-mail,Tags,City',
        `${PHONE(2)},Ali Veli,ali@example.com,vip;istanbul,Istanbul`,
        `${PHONE(3)},Ayşe Kaya,,vip,Ankara`,
        'not-a-phone,Broken Row,,,',
    ].join('\n');
    // The file input only reacts once the page has hydrated: retry until
    // the chosen file shows up.
    await page.waitForLoadState('networkidle');
    await expect(async () => {
        await page.locator('input[type="file"]').setInputFiles({
            name: `customers-${RUN}.csv`,
            mimeType: 'text/csv',
            buffer: Buffer.from(csv),
        });
        await expect(
            page.getByText(`customers-${RUN}.csv`).first(),
        ).toBeVisible({ timeout: 2_000 });
    }).toPass({ timeout: 20_000 });
    await page.getByRole('button', { name: 'Upload and continue' }).click();

    await expect(page).toHaveURL(/\/contacts\/import\/[0-9a-f-]{36}$/);
    await expect(
        page.getByText('3 rows found.', { exact: false }),
    ).toBeVisible();
    // Phone, name, e-mail and tags are recognised; City becomes a custom field.
    await expect(
        page.getByRole('combobox', { name: 'Import column Phone as' }),
    ).toHaveText('Phone number');
    await page.getByRole('combobox', { name: 'Import column City as' }).click();
    await page.getByRole('option', { name: 'Custom field' }).click();
    await expect(
        page.getByLabel('Custom field name for column City'),
    ).toHaveValue('city');
    const extra = page.getByLabel('Tag every imported contact');
    await extra.fill('imported');
    await extra.press('Enter');
    await page.getByRole('button', { name: 'Start import' }).click();

    await expect(page.getByTestId('import-status')).toHaveText('Completed', {
        timeout: 60_000,
    });
    await expect(page.getByTestId('import-created')).toHaveText('2');
    await expect(page.getByTestId('import-failed')).toHaveText('1');
    const [report] = await Promise.all([
        page.waitForEvent('download'),
        page.getByRole('link', { name: 'Download failed rows' }).click(),
    ]);
    const text = await readFile((await report.path())!, 'utf8');
    expect(text).toContain('not-a-phone');

    await page.getByRole('link', { name: 'View contacts' }).click();
    await page.getByLabel('Search', { exact: true }).fill(PHONE(3));
    await page.getByLabel('Search', { exact: true }).press('Enter');
    await expect(page.getByTestId('contact-row')).toHaveCount(1);
    await expect(page.getByTestId('contact-row')).toContainText('imported');
    await expect(page.getByTestId('contact-row')).toContainText('Ayşe Kaya');
});

test('contacts are added to a group in bulk', async () => {
    await page.goto('/contacts/groups');
    await page.getByRole('button', { name: 'New group' }).click();
    const dialog = page.getByRole('dialog');
    await dialog.getByLabel('Name', { exact: true }).fill(`Newsletter ${RUN}`);
    await dialog.getByRole('button', { name: 'Create group' }).click();
    await expect(
        page.getByRole('cell', { name: `Newsletter ${RUN}`, exact: true }),
    ).toBeVisible();

    await page.goto(`/contacts?q=${encodeURIComponent(PREFIX)}`);
    await expect(page.getByTestId('contact-row')).toHaveCount(3);
    // Bulk action bar: select the page, add everyone to the group.
    await page
        .getByRole('checkbox', { name: 'Select all on this page' })
        .click();
    await expect(page.getByText('3 selected')).toBeVisible();
    await page.getByRole('button', { name: 'Add to group' }).click();
    const picker = page.getByRole('dialog').filter({ hasText: 'Add to group' });
    await picker.getByRole('combobox', { name: 'Group' }).click();
    await page.getByRole('option', { name: `Newsletter ${RUN}` }).click();
    await page.getByRole('button', { name: 'Apply' }).click();
    await expect(
        page.getByText(`Added 3 contacts to Newsletter ${RUN}.`),
    ).toBeVisible();

    await page.goto('/contacts/groups');
    await expect(
        page.getByRole('link', {
            name: `3 members of Newsletter ${RUN}; show them`,
        }),
    ).toBeVisible();
});

test('a segment shows its live count and is saved', async () => {
    await page.goto('/contacts/segments');
    await page.getByRole('link', { name: 'New segment' }).click();
    await page.getByLabel('Name', { exact: true }).fill(`VIP ${RUN}`);
    // The first rule is "Tag has …".
    await page.getByLabel('Value', { exact: true }).fill('vip');
    // Narrow it to the contacts of this run with a nested rule group.
    await page.getByRole('button', { name: 'Add rule group' }).click();
    await expect(page.getByTestId('segment-group')).toHaveCount(2);
    const second = page.getByTestId('segment-rule').nth(1);
    await second.getByRole('combobox', { name: 'Field' }).click();
    await page.getByRole('option', { name: 'Phone number' }).click();
    await second.getByRole('combobox', { name: /Condition on/ }).click();
    await page.getByRole('option', { name: 'starts with' }).click();
    await second.getByLabel('Value', { exact: true }).fill(PREFIX);

    await expect(page.getByTestId('segment-count')).toHaveText('3');
    await page.getByRole('button', { name: 'Save segment' }).click();
    await expect(page).toHaveURL(/\/contacts\/segments\/[0-9a-f-]{36}$/);
    await expect(page.getByText('3 contacts match right now.')).toBeVisible();
});

test('the filtered list exports as CSV', async () => {
    await page.goto(`/contacts?q=${encodeURIComponent(PREFIX)}`);
    await page.getByRole('button', { name: 'Export' }).click();
    const [download] = await Promise.all([
        page.waitForEvent('download'),
        page.getByRole('menuitem', { name: 'CSV (.csv)' }).click(),
    ]);
    expect(download.suggestedFilename()).toBe('messora-contacts.csv');
    const text = await readFile((await download.path())!, 'utf8');
    const lines = text.trim().split('\n');
    expect(lines[0]).toMatch(/^phone,name,/);
    expect(lines).toHaveLength(4);
    expect(text).toContain(PHONE(2));
});
