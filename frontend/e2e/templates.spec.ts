import { execFileSync } from 'node:child_process';
import { expect, test, type Page } from '@playwright/test';

/**
 * The template builder and time-limited offers. Needs identity.spec.ts to
 * have run first (the user below reuses its password hash).
 *
 * The E2E stack has no Meta mock (see campaigns.spec.ts): the seeded
 * account is disabled, so a template that passes the server's component
 * validation is refused with "This WhatsApp account is disabled." instead
 * of reaching Meta. That refusal is what proves the builder's JSON passed
 * the server's checks; the exact JSON Meta receives is covered by the
 * integration tests (internal/app/offers_integration_test.go).
 */

const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';
const USER = { email: 'tina@example.com', password: 'Another-strong-pass-73' };
const PERMISSIONS = [
    'channels.read',
    'templates.read',
    'templates.manage',
    'campaigns.read',
];
const TAG = String(Date.now()).slice(-6);
const ACCOUNT = `E2E Templates ${TAG}`;

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

async function signIn(page: Page) {
    await page.goto('/login');
    await page.getByLabel('E-mail').fill(USER.email);
    await page.getByLabel('Password').fill(USER.password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(
        page.getByRole('heading', { name: 'Welcome, Tina' }),
    ).toBeVisible();
}

function watchErrors(page: Page) {
    const errors: string[] = [];
    page.on('pageerror', (e) => errors.push(e.message));
    return () => expect(errors, 'browser errors').toEqual([]);
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
        WITH r AS (INSERT INTO roles (key, name, rank) VALUES ('e2e_templates', 'Templates (E2E)', 25)
                   ON CONFLICT (key) DO UPDATE SET name = EXCLUDED.name RETURNING id),
             p AS (INSERT INTO role_permissions (role_id, permission_key)
                   SELECT r.id, k FROM r, unnest(ARRAY['${PERMISSIONS.join("','")}']) k ON CONFLICT DO NOTHING),
             u AS (INSERT INTO users (email, name, password_hash)
                   SELECT '${USER.email}', 'Tina Templates', password_hash FROM users WHERE email = 'ari@example.com'
                   ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name RETURNING id)
        INSERT INTO user_roles (user_id, role_id) SELECT u.id, r.id FROM u, r ON CONFLICT DO NOTHING;`);
    // A disabled account with an approved limited-time offer template.
    sql(`
        WITH a AS (INSERT INTO whatsapp_accounts (name, waba_id, access_token_enc, status)
                   VALUES ('${ACCOUNT}', '9301${TAG}', '\\x00', 'disabled') RETURNING id)
        INSERT INTO message_templates (account_id, meta_template_id, name, language, category, status, components, quality_score)
        SELECT id, 'e2e-lto-${TAG}', 'e2e_lto_${TAG}', 'en_US', 'MARKETING', 'APPROVED',
               '[{"type":"LIMITED_TIME_OFFER","limited_time_offer":{"text":"Hurry up!","has_expiration":true}},
                 {"type":"BODY","text":"Hi {{1}}, use {{2}} today.","example":{"body_text":[["Ece","XMAS"]]}},
                 {"type":"BUTTONS","buttons":[{"type":"COPY_CODE","example":"XMAS"},{"type":"URL","text":"Shop","url":"https://shop.test/"}]}]'::jsonb,
               'GREEN'
        FROM a;`);
});

test('the builder makes a limited-time offer template and checks Meta’s limits', async ({
    page,
}) => {
    const noErrors = watchErrors(page);
    await signIn(page);
    await page.goto('/templates');
    // The seeded template shows its special type and quality.
    const card = page
        .getByTestId('template')
        .filter({ hasText: `e2e_lto_${TAG}` });
    await expect(card).toContainText('Limited-time offer');
    await expect(card).toContainText('Quality: green');

    await page.getByTestId('new-template').click();
    await expect(page).toHaveURL(/\/templates\/new$/);
    const builder = page.getByTestId('template-builder');
    await builder.getByRole('radio', { name: /Limited-time offer/ }).click();
    // Marketing only: the category is fixed.
    await expect(builder.getByLabel('Category')).toBeDisabled();

    await builder.getByLabel('Account').click();
    await page.getByRole('option', { name: ACCOUNT }).click();
    await builder.getByLabel('Name').fill(`e2e_offer_${TAG}`);
    await builder.getByLabel('Offer text').fill('Ends tonight, hurry up!');
    const body = builder.getByLabel('Body', { exact: true });
    await body.fill('Hello ');
    await builder.getByRole('button', { name: 'Add variable' }).click();
    await expect(body).toHaveValue('Hello {{1}}');
    await body.fill('Hello {{1}}, your code is ready.');
    await builder.getByLabel('Body {{1}}').fill('Ece');
    await builder.getByLabel('Example coupon').fill('XMAS25');
    await builder.getByLabel('Button text').fill('Shop now');
    await builder
        .getByLabel('Address', { exact: true })
        .fill('https://shop.test/xmas');

    // The live preview: offer row with countdown, coupon and buttons.
    const preview = page.getByTestId('template-preview');
    await expect(preview.getByTestId('preview-offer')).toContainText(
        'Offer ends in',
    );
    await expect(preview.getByTestId('preview-offer')).toContainText('XMAS25');
    await expect(preview).toContainText('Hello Ece, your code is ready.');
    await expect(preview).toContainText('Copy code');

    // Meta's 16-character limit on the offer text is checked before sending.
    await page.getByTestId('template-submit').click();
    await expect(page.locator('#tpl-lto-text-error')).toBeVisible();
    await builder.getByLabel('Offer text').fill('Hurry up!');

    // Valid: the server's component checks pass and only the disabled
    // account stops it from reaching Meta.
    await page.getByTestId('template-submit').click();
    await expect(
        page.getByRole('alert').filter({ hasText: 'disabled' }),
    ).toBeVisible();
    await expect(page).toHaveURL(/\/templates\/new$/);
    noErrors();
});

test('the carousel builder keeps cards identical and previews them side by side', async ({
    page,
}) => {
    const noErrors = watchErrors(page);
    await signIn(page);
    await page.goto('/templates/new');
    const builder = page.getByTestId('template-builder');
    await builder.getByRole('radio', { name: /Carousel/ }).click();
    const cards = page.getByTestId('preview-cards').locator('li');
    await expect(cards).toHaveCount(2);
    await builder.getByTestId('add-card').click();
    await expect(cards).toHaveCount(3);
    await builder.getByLabel('Body', { exact: true }).fill('Our picks for you');
    // Every card needs its sample image before Meta's review.
    await builder.getByLabel('Account').click();
    await page.getByRole('option', { name: ACCOUNT }).click();
    await builder.getByLabel('Name').fill(`e2e_cards_${TAG}`);
    await page.getByTestId('template-submit').click();
    await expect(page.locator('#tpl-card-0-media-error')).toBeVisible();
    noErrors();
});

test('offers: create with a countdown preview, then end it', async ({
    page,
}) => {
    const noErrors = watchErrors(page);
    await signIn(page);
    await page.goto('/templates');
    await page.getByRole('link', { name: 'Offers' }).click();
    await expect(
        page.getByRole('heading', { name: 'Offers', level: 1 }),
    ).toBeVisible();

    await page.getByRole('button', { name: 'New offer' }).click();
    const form = page.getByTestId('offer-form');
    await form.getByLabel('Title').fill(`Winter sale ${TAG}`);
    await form.getByLabel('Discount').fill('25% off');
    await form.getByLabel('Coupon code').fill('WINTER25');
    await form.getByLabel('Template').click();
    await page
        .getByRole('option', { name: new RegExp(`e2e_lto_${TAG}`) })
        .click();
    // The LTO template's WhatsApp countdown, filled from the offer.
    await expect(form.getByTestId('preview-offer')).toContainText(
        /Offer ends in \d+ d/,
    );
    await expect(form.getByTestId('preview-offer')).toContainText('WINTER25');
    await expect(form).toContainText('WhatsApp shows the countdown');
    await form.getByRole('button', { name: 'Create offer' }).click();

    const row = page
        .getByTestId('offer')
        .filter({ hasText: `Winter sale ${TAG}` });
    await expect(row).toContainText('Scheduled');
    await expect(row).toContainText('WINTER25');
    expect(
        sql(
            `SELECT status || ' ' || coupon_code FROM offers WHERE title = 'Winter sale ${TAG}'`,
        ),
    ).toBe('scheduled WINTER25');

    await row
        .getByRole('button', { name: `End Winter sale ${TAG} now` })
        .click();
    await expect(row).toContainText('Ended');
    expect(
        sql(`SELECT status FROM offers WHERE title = 'Winter sale ${TAG}'`),
    ).toBe('expired');
    noErrors();
});
