import { execFileSync } from 'node:child_process';
import { createHmac, timingSafeEqual } from 'node:crypto';
import { createServer, type IncomingHttpHeaders, type Server } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, test, type Page } from '@playwright/test';

/**
 * Phase 17 WP4: every workspace has its own API address and its own
 * outgoing webhooks. The API keys page shows the workspace's base URL, the
 * reference at /docs/api names it as the server, and a test event reaches
 * a receiver running next to the tests, signed with the endpoint's secret.
 * Needs identity.spec.ts to have run first (the user below reuses its
 * password hash).
 */

const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';
const USER = { email: 'ivy@example.com', password: 'Another-strong-pass-73' };
const PERMISSIONS = ['api_keys.manage', 'webhooks.manage'];

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
        page.getByRole('heading', { name: 'Welcome, Ivy' }),
    ).toBeVisible();
}

type Received = { headers: IncomingHttpHeaders; body: string };

/** A webhook receiver on the host, reachable from the stack's worker. */
async function receiver(): Promise<{
    server: Server;
    port: number;
    got: Received[];
}> {
    const got: Received[] = [];
    const server = createServer((req, res) => {
        let body = '';
        req.on('data', (c: Buffer) => (body += c.toString('utf8')));
        req.on('end', () => {
            got.push({ headers: req.headers, body });
            res.writeHead(200, { 'Content-Type': 'application/json' });
            res.end('{"ok":true}');
        });
    });
    await new Promise<void>((resolve) => server.listen(0, '0.0.0.0', resolve));
    return { server, port: (server.address() as AddressInfo).port, got };
}

/** Messora-Signature check, as docs/WEBHOOKS.md tells integrators. */
function verify(secret: string, header: string, body: string): boolean {
    const parts = header.split(',').map((p) => p.split('='));
    const t = Number(parts.find(([k]) => k === 't')?.[1]);
    if (!t || Math.abs(Date.now() / 1000 - t) > 300) return false;
    const want = createHmac('sha256', secret).update(`${t}.${body}`).digest();
    return parts
        .filter(([k]) => k === 'v1')
        .some(([, v]) => {
            const got = Buffer.from(v ?? '', 'hex');
            return got.length === want.length && timingSafeEqual(got, want);
        });
}

test.describe.configure({ mode: 'serial' });

test.beforeAll(() => {
    const hash = sql(
        `SELECT password_hash FROM users WHERE email = 'ari@example.com'`,
    );
    expect(hash, 'identity.spec.ts must run first').toMatch(/^\$argon2id\$/);
    sql(`
        WITH r AS (INSERT INTO roles (key, name, rank) VALUES ('e2e_integrator', 'Integrator (E2E)', 25)
                   ON CONFLICT (key) DO UPDATE SET name = EXCLUDED.name RETURNING id),
             p AS (INSERT INTO role_permissions (role_id, permission_key)
                   SELECT r.id, k FROM r, unnest(ARRAY['${PERMISSIONS.join("','")}']) k ON CONFLICT DO NOTHING),
             u AS (INSERT INTO users (email, name, password_hash, email_verified_at)
                   SELECT '${USER.email}', 'Ivy Integrator', password_hash, now() FROM users WHERE email = 'ari@example.com'
                   ON CONFLICT (email) DO UPDATE SET name = EXCLUDED.name RETURNING id)
        INSERT INTO tenant_memberships (user_id, role_id) SELECT u.id, r.id FROM u, r ON CONFLICT DO NOTHING;`);
});

test("the API keys page and the reference show the workspace's own address", async ({
    page,
    request,
    baseURL,
}) => {
    await signIn(page);
    await page.goto('/api-keys');
    const base = (await page.getByTestId('api-base-url').textContent())?.trim();
    // Tenant #1's primary address: localhost or its sub-domain.
    expect(base).toMatch(/^http:\/\/([a-z0-9-]+\.)*localhost:\d+\/api\/v1$/);
    await expect(page.getByTestId('api-base')).toContainText(
        `curl ${base}/api-keys/whoami`,
    );

    // The document served at this address names it first (Scalar's
    // samples and "try it" use the first server).
    const origin = new URL(baseURL!).origin;
    const doc = await (await request.get('/api/v1/openapi.json')).json();
    expect(doc.servers[0].url).toBe(`${origin}/api/v1`);
    expect(doc.info.description).toContain(
        `curl ${origin}/api/v1/api-keys/whoami`,
    );
    expect(doc.paths['/platform/tenants']).toBeUndefined();
    // The platform address shows the per-workspace pattern.
    const platform = process.env.E2E_PLATFORM_URL;
    if (platform) {
        const generic = await (
            await request.get(`${platform}/api/v1/openapi.json`)
        ).json();
        expect(generic.servers[0].url).toMatch(/\{workspace\}/);
    }

    await page.goto('/docs/api');
    await expect(
        page.getByRole('heading', { name: 'Messora API' }).first(),
    ).toBeVisible();
    await expect(page.getByText('Your API address').first()).toBeVisible();
});

test('a test event reaches a local receiver, signed', async ({ page }) => {
    const rc = await receiver();
    try {
        await signIn(page);
        await page.goto('/settings/webhooks');
        await page.getByRole('button', { name: 'Add endpoint' }).click();
        const dialog = page.getByRole('dialog');
        await dialog
            .getByLabel('URL', { exact: true })
            .fill(`http://host.docker.internal:${rc.port}/hooks/messora`);
        await dialog.getByLabel('contact.created').check();
        await dialog.getByRole('button', { name: 'Add endpoint' }).click();
        const secret = await dialog.getByLabel('Signing secret').inputValue();
        expect(secret).toMatch(/^whsec_/);
        await dialog.getByRole('button', { name: 'Done' }).click();

        const row = page
            .getByRole('row')
            .filter({ hasText: `:${rc.port}/hooks/messora` });
        await expect(row).toBeVisible();
        await row.getByRole('button', { name: 'More actions' }).click();
        await page.getByRole('menuitem', { name: 'Send test event' }).click();

        await expect
            .poll(() => rc.got.length, { timeout: 30_000 })
            .toBeGreaterThan(0);
        const hit = rc.got[0]!;
        expect(hit.headers['messora-event-type']).toBe('webhook.test');
        expect(
            verify(secret, String(hit.headers['messora-signature']), hit.body),
            'signature verifies with the endpoint secret',
        ).toBe(true);
        expect(JSON.parse(hit.body).type).toBe('webhook.test');

        // The delivery log shows it as succeeded.
        await row.getByRole('button', { name: 'Deliveries' }).click();
        await expect(
            page.getByRole('dialog').getByText('succeeded').first(),
        ).toBeVisible();
    } finally {
        rc.server.close();
    }
});
