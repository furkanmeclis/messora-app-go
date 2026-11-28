import { createHmac } from 'node:crypto';
import { execFileSync } from 'node:child_process';
import { expect, test, type Page } from '@playwright/test';

/**
 * Settings → WhatsApp / Meta → "Connect with Meta" (phase 17 WP6): Messora
 * as a Meta Tech Provider with Embedded Signup. Needs identity.spec.ts to
 * have run first (the user below reuses its password hash).
 *
 * Nothing reaches Facebook. The stack's Graph API is the fake Meta
 * (docker-compose.e2e.yml, `messora fake-meta`); the browser's Meta SDK is
 * replaced by a stand-in served for connect.facebook.net, whose popup is a
 * facebook.com frame that posts the WA_EMBEDDED_SIGNUP message exactly as
 * Meta's does. So the page's CSP, its origin check, the code exchange and
 * every server step run for real.
 *
 * The platform switch is flipped with `messora tech-provider` (the platform
 * admin's command until WP2's admin panel has its form) and switched off
 * again at the end, so the other specs see the method off.
 */

const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';
const USER = { email: 'tess@example.com', password: 'Another-strong-pass-73' };
const PERMISSIONS = ['channels.read', 'channels.manage'];
const APP_ID = '1234'; // the fake Meta's app (FAKE_META_APP_ID)
const APP_SECRET = 'e2e-tech-provider-app-secret-0001';
const WABA = '410000000001';
const PHONE = '410000000011';

function compose(args: string[], env: Record<string, string> = {}): string {
    return execFileSync('docker', ['compose', '-p', PROJECT, ...args], {
        cwd: '..',
        encoding: 'utf8',
        env: { ...process.env, ...env },
    }).trim();
}

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

/** `messora tech-provider …` in the API container (owner credentials). */
function techProvider(...args: string[]): string {
    return compose(
        [
            'exec',
            '-T',
            '-e',
            `MESSORA_TP_APP_SECRET=${APP_SECRET}`,
            'messora-api',
            '/usr/local/bin/messora',
            'tech-provider',
            ...args,
        ],
        {},
    );
}

function verifyToken(): string {
    const m = techProvider('show').match(/Verify token:\s+([a-z2-7]{52})/);
    if (!m?.[1]) throw new Error('no verify token');
    return m[1];
}

/** The stand-in for Meta's SDK and its popup. */
async function fakeMeta(page: Page) {
    await page.route('https://connect.facebook.net/**', (route) =>
        route.fulfill({
            contentType: 'application/javascript',
            body: `
                window.FB = {
                    init(o) { window.__fbInit = o; },
                    login(cb, opts) {
                        window.__fbLogin = opts;
                        const f = document.createElement('iframe');
                        f.style.display = 'none';
                        f.src = 'https://www.facebook.com/e2e-embedded-signup';
                        f.onload = () => cb({ status: 'connected', authResponse: { code: 'e2e-es.${WABA}.${PHONE}' } });
                        document.body.appendChild(f);
                    },
                };
                setTimeout(() => window.fbAsyncInit && window.fbAsyncInit(), 0);
            `,
        }),
    );
    await page.route('https://www.facebook.com/e2e-embedded-signup', (route) =>
        route.fulfill({
            contentType: 'text/html',
            body: `<!doctype html><script>
                parent.postMessage(JSON.stringify({
                    type: 'WA_EMBEDDED_SIGNUP', event: 'FINISH', version: 4,
                    data: { phone_number_id: '${PHONE}', waba_id: '${WABA}', business_id: '410000000099' },
                }), '*');
            </script>`,
        }),
    );
}

async function signIn(page: Page) {
    await page.goto('/login');
    await page.getByLabel('E-mail').fill(USER.email);
    await page.getByLabel('Password', { exact: true }).fill(USER.password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(
        page.getByRole('heading', { name: 'Welcome, Tess' }),
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
        WITH r AS (INSERT INTO roles (key, name, rank) VALUES ('e2e_tp', 'Tech Provider (E2E)', 25)
                   ON CONFLICT (key) DO UPDATE SET name = EXCLUDED.name RETURNING id),
             p AS (INSERT INTO role_permissions (role_id, permission_key)
                   SELECT r.id, k FROM r, unnest(ARRAY['${PERMISSIONS.join("','")}']) k ON CONFLICT DO NOTHING),
             u AS (INSERT INTO users (email, name, password_hash)
                   SELECT '${USER.email}', 'Tess Provider', password_hash FROM users WHERE email = 'ari@example.com'
                   ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name RETURNING id)
        INSERT INTO user_roles (user_id, role_id) SELECT u.id, r.id FROM u, r ON CONFLICT DO NOTHING;`);
});

test.afterAll(() => {
    // The other specs expect the method off.
    techProvider('set', '-disable');
});

test('with the switch off the Tech Provider card stays disabled', async ({
    page,
    request,
}) => {
    techProvider('set', '-disable');
    await signIn(page);
    await page.goto('/settings/meta');
    await expect(page.getByTestId('method-tech-provider')).toBeDisabled();
    await expect(page.getByTestId('method-tech-provider')).toContainText(
        'Coming soon',
    );
    await expect(page.getByTestId('tech-provider-card')).toHaveCount(0);
    // The platform webhook does not exist while off.
    const off = await request.get(
        '/webhooks/meta/tp?hub.mode=subscribe&hub.verify_token=x&hub.challenge=1',
    );
    expect(off.status()).toBe(404);
    // The CSP of other pages never allows Meta's SDK.
    const other = await request.get('/settings');
    expect(other.headers()['content-security-policy'] ?? '').not.toContain(
        'facebook',
    );
});

test('Embedded Signup with the switch on connects the channel', async ({
    page,
    request,
}) => {
    const out = techProvider(
        'set',
        '-enable',
        '-app-id',
        APP_ID,
        '-configuration-id',
        '555000111',
    );
    expect(out).toContain('Enabled:            true (ready: true)');
    const errors: string[] = [];
    page.on('pageerror', (e) => errors.push(e.message));
    const blocked: string[] = [];
    page.on('console', (m) => {
        if (/Content Security Policy/i.test(m.text())) blocked.push(m.text());
    });
    await fakeMeta(page);
    await signIn(page);
    const res = await page.goto('/settings/meta');
    // Only this page allows the SDK.
    const csp = res?.headers()['content-security-policy'] ?? '';
    expect(csp).toContain('https://connect.facebook.net');
    expect(csp).toContain('frame-src');

    await expect(page.getByTestId('method-tech-provider')).toBeEnabled();
    await page.getByTestId('method-tech-provider').click();
    const card = page.getByTestId('tech-provider-card');
    await expect(card).toBeVisible();
    await expect(card).toContainText('whatsapp_business_messaging');
    await card.getByTestId('tp-pin-own').click();
    await card.getByLabel('6-digit PIN').fill('246810');
    await card.getByTestId('tp-connect').click();

    await expect(page.getByTestId('tp-success')).toBeVisible({
        timeout: 30_000,
    });
    await expect(page.getByTestId('tp-success')).toContainText(
        'E2E İşletme is connected.',
    );
    // The SDK got Messora's app and the Embedded Signup options.
    const init = await page.evaluate(
        () =>
            (window as unknown as { __fbInit: Record<string, unknown> })
                .__fbInit,
    );
    expect(init.appId).toBe(APP_ID);
    const login = await page.evaluate(
        () =>
            (window as unknown as { __fbLogin: Record<string, unknown> })
                .__fbLogin,
    );
    expect(login).toMatchObject({
        config_id: '555000111',
        response_type: 'code',
        override_default_response_type: true,
    });

    // The channel exists, routed to this workspace, with the method set.
    expect(
        sql(
            `SELECT connection_method || ':' || status FROM whatsapp_accounts WHERE waba_id = '${WABA}'`,
        ),
    ).toBe('tech_provider:active');
    expect(
        sql(`SELECT tenant_id FROM meta_routes WHERE waba_id = '${WABA}'`),
    ).toBe('00000000-0000-0000-0000-000000000001');
    await page.goto('/channels');
    await expect(page.getByText('E2E İşletme').first()).toBeVisible();

    // The platform webhook through the edge proxy: Meta's handshake with
    // the platform verify token, and an event signed with the app secret.
    const token = verifyToken();
    const hs = await request.get(
        `/webhooks/meta/tp?hub.mode=subscribe&hub.verify_token=${token}&hub.challenge=7788`,
    );
    expect(hs.status()).toBe(200);
    expect(await hs.text()).toBe('7788');
    const body = JSON.stringify({
        object: 'whatsapp_business_account',
        entry: [
            {
                id: WABA,
                changes: [
                    {
                        field: 'messages',
                        value: {
                            messaging_product: 'whatsapp',
                            metadata: {
                                display_phone_number: '15550000000',
                                phone_number_id: PHONE,
                            },
                            contacts: [
                                {
                                    wa_id: '905551230001',
                                    profile: { name: 'TP' },
                                },
                            ],
                            messages: [
                                {
                                    from: '905551230001',
                                    id: 'wamid.e2e-tp-1',
                                    timestamp: String(
                                        Math.floor(Date.now() / 1000),
                                    ),
                                    type: 'text',
                                    text: { body: 'merhaba tech provider' },
                                },
                            ],
                        },
                    },
                ],
            },
        ],
    });
    const sig =
        'sha256=' + createHmac('sha256', APP_SECRET).update(body).digest('hex');
    const wrong = await request.post('/webhooks/meta/tp', {
        data: body,
        headers: {
            'Content-Type': 'application/json',
            'X-Hub-Signature-256': 'sha256=00',
        },
    });
    expect(wrong.status()).toBe(401);
    const hook = await request.post('/webhooks/meta/tp', {
        data: body,
        headers: {
            'Content-Type': 'application/json',
            'X-Hub-Signature-256': sig,
        },
    });
    expect(hook.status()).toBe(200);
    await expect
        .poll(() =>
            sql(`SELECT count(*) FROM messages WHERE wamid = 'wamid.e2e-tp-1'`),
        )
        .toBe('1');

    // Disconnect: the route goes, the history stays.
    await page.goto('/settings/meta');
    await page.getByTestId('method-tech-provider').click();
    const accounts = page.getByTestId('tp-accounts');
    await accounts.getByRole('button', { name: 'Disconnect' }).click();
    await page
        .getByRole('alertdialog')
        .getByRole('button', { name: 'Disconnect' })
        .click();
    await expect(
        page.getByText("Disconnected from Messora's Meta app."),
    ).toBeVisible();
    expect(
        sql(`SELECT count(*) FROM meta_routes WHERE waba_id = '${WABA}'`),
    ).toBe('0');
    expect(
        sql(`SELECT status FROM whatsapp_accounts WHERE waba_id = '${WABA}'`),
    ).toBe('disabled');

    expect(errors, 'browser errors').toEqual([]);
    expect(blocked, 'CSP violations').toEqual([]);
});
