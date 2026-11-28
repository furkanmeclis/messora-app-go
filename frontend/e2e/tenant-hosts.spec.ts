import { request as httpRequest } from 'node:http';
import { expect, test } from '@playwright/test';

/**
 * Companies on their own sub-domains, through the edge exactly as in
 * production (Caddy, EDGE_MODE=proxy): tenant #1 ("default") and a second
 * company ("beta", opened by scripts/e2e.sh) on <slug>.<E2E_BASE_DOMAIN>.
 * A session belongs to the host that issued it, unknown hosts reach no
 * company, the edge decides the forwarded host itself and refuses
 * cross-origin realtime connections. Runs after identity.spec.ts, which
 * creates the agent signing in here.
 */

const AGENT = { email: 'ari@example.com', password: 'Another-strong-pass-73' };
const BASE = process.env.E2E_BASE_DOMAIN ?? 'e2e.localhost';
const PORT = new URL(process.env.E2E_BASE_URL ?? 'http://localhost:8090').port;
const hostOf = (slug: string) => `${slug}.${BASE}:${PORT}`;
const A = hostOf('default');
const B = hostOf('beta');

type Raw = { status: number; body: string; upgraded: boolean };

/**
 * A request to the edge on the loopback address with an explicit Host
 * header (Node, unlike Chromium, may not resolve *.localhost), so headers
 * a browser would never send can be tried too.
 */
function raw(
    path: string,
    host: string,
    headers: Record<string, string> = {},
): Promise<Raw> {
    return new Promise((resolve, reject) => {
        const req = httpRequest(
            {
                host: '127.0.0.1',
                port: Number(PORT),
                path,
                headers: { host, ...headers },
            },
            (res) => {
                let body = '';
                res.setEncoding('utf8');
                res.on('data', (c) => (body += c));
                res.on('end', () =>
                    resolve({
                        status: res.statusCode ?? 0,
                        body,
                        upgraded: false,
                    }),
                );
            },
        );
        req.on('upgrade', (res, socket) => {
            socket.destroy();
            resolve({ status: res.statusCode ?? 0, body: '', upgraded: true });
        });
        req.on('error', reject);
        req.setTimeout(15_000, () => req.destroy(new Error('timeout')));
        req.end();
    });
}

function websocket(host: string, origin?: string) {
    return raw('/connection/websocket', host, {
        connection: 'Upgrade',
        upgrade: 'websocket',
        'sec-websocket-version': '13',
        'sec-websocket-key': Buffer.from('messora-e2e-key!').toString('base64'),
        ...(origin ? { origin } : {}),
    });
}

test('a session belongs to the company address that issued it', async ({
    page,
    context,
}) => {
    await page.goto(`http://${A}/login`);
    await page.getByLabel('E-mail').fill(AGENT.email);
    await page.getByLabel('Password', { exact: true }).fill(AGENT.password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(
        page.getByRole('heading', { name: 'Welcome, Ari' }),
    ).toBeVisible();
    await expect(page).toHaveURL(
        new RegExp(`^http://${A.replace(/\./g, '\\.')}/`),
    );

    // The other company's address knows nobody: its sign-in page names it.
    await page.goto(`http://${B}/`);
    await expect(page).toHaveURL(/\/login/);
    await expect(
        page.getByRole('heading', { name: 'Sign in to beta' }),
    ).toBeVisible();

    // A's session cookie, replayed on B's host, is refused by the API.
    const issued = (await context.cookies(`http://${A}/`)).filter((c) =>
        /messora_session/.test(c.name),
    );
    expect(issued.length, 'session cookie on the company host').toBeGreaterThan(
        0,
    );
    await context.addCookies(
        issued.map((c) => ({ ...c, domain: `beta.${BASE}`, path: '/' })),
    );
    await page.goto(`http://${B}/`);
    await expect(page).toHaveURL(/\/login/);
    await expect(
        page.getByRole('heading', { name: 'Welcome, Ari' }),
    ).toHaveCount(0);

    // A's credentials do not open B either.
    await page.getByLabel('E-mail').fill(AGENT.email);
    await page.getByLabel('Password', { exact: true }).fill(AGENT.password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(
        page.getByText('E-mail or password is incorrect'),
    ).toBeVisible();
    await expect(page).toHaveURL(/\/login/);

    // And A's own address still has its session.
    await page.goto(`http://${A}/`);
    await expect(
        page.getByRole('heading', { name: 'Welcome, Ari' }),
    ).toBeVisible();
});

test('the API answers only on company addresses', async () => {
    const unknown = await raw('/api/v1/system/status', hostOf('nobody'));
    expect(unknown.status).toBe(404);
    expect(unknown.body).toContain('TENANT_NOT_FOUND');
    // A known company without credentials: authentication, not routing.
    expect((await raw('/api/v1/system/status', B)).status).toBe(401);
});

test('the edge sets the forwarded host itself', async () => {
    // A forged X-Forwarded-Host must not move the page to another company:
    // Caddy replaces it with the Host it received.
    const page = await raw('/login', A, { 'x-forwarded-host': B });
    expect(page.status).toBe(200);
    expect(page.body).toContain('Sign in to Acme E2E');
    expect(page.body).not.toContain('Sign in to beta');
    // The BFF's host header is internal; from outside it changes nothing.
    const forged = await raw('/login', A, { 'x-messora-tenant-host': B });
    expect(forged.body).toContain('Sign in to Acme E2E');
});

test('realtime connections are accepted only from their own origin', async () => {
    // Same origin, on a company sub-domain and on tenant #1's own address.
    expect((await websocket(A, `http://${A}`)).upgraded).toBe(true);
    expect(
        (await websocket(`localhost:${PORT}`, `http://localhost:${PORT}`))
            .upgraded,
    ).toBe(true);
    // Another company's page, or any other site, may not use this host's
    // realtime endpoint (it is refused by the edge before Centrifugo).
    for (const origin of [
        `http://${B}`,
        'https://evil.example',
        `http://${A}.evil.example`,
    ]) {
        const res = await websocket(A, origin);
        expect(res.upgraded, origin).toBe(false);
        expect(res.status, origin).toBe(403);
    }
});

test('the platform admin host is served', async ({ page }) => {
    // The platform admin panel arrives with Phase 17 WP2, whose e2e.sh sets
    // E2E_ADMIN_URL.
    const admin = process.env.E2E_ADMIN_URL;
    test.skip(!admin, 'no platform admin host on this stack');
    const res = await page.goto(admin + '/login');
    expect(res?.ok()).toBe(true);
    await expect(page).toHaveURL(
        new RegExp(`^${admin!.replace(/\./g, '\\.')}/login`),
    );
    await expect(page.getByLabel('E-mail')).toBeVisible();
});
