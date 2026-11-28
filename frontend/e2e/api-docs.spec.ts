import { expect, test } from '@playwright/test';

/**
 * The public API reference: reachable without signing in, rendered by the
 * bundled Scalar from /api/v1/openapi.json, and loading nothing from other
 * origins (no CDN, fonts or telemetry) under the panel's CSP.
 */

test('the public OpenAPI document is anonymous and API-key only', async ({
    request,
}) => {
    const res = await request.get('/api/v1/openapi.json');
    expect(res.status()).toBe(200);
    const doc = await res.json();
    expect(doc.info.title).toBe('Messora API');
    expect(doc.paths['/messages'].post).toBeTruthy();
    for (const internal of [
        '/auth/login',
        '/auth/signup',
        '/platform/tenants',
        '/users',
        '/roles',
    ]) {
        expect(doc.paths[internal]).toBeUndefined();
    }
    expect(Object.keys(doc.components.securitySchemes)).toEqual(['apiKey']);
});

test('the API reference renders, searches and shows code samples', async ({
    page,
    baseURL,
}) => {
    const origin = new URL(baseURL!).origin;
    const foreign: string[] = [];
    const problems: string[] = [];
    page.on('request', (r) => {
        const u = new URL(r.url());
        if (u.protocol.startsWith('http') && u.origin !== origin)
            foreign.push(r.url());
    });
    page.on('console', (m) => {
        if (m.type() === 'error' && /Content Security Policy/i.test(m.text()))
            problems.push(m.text());
    });
    page.on('pageerror', (e) => problems.push(e.message));

    // No session cookie: the page must not redirect to the sign-in page.
    await page.goto('/docs/api');
    await expect(page).toHaveURL(/\/docs\/api/);
    await expect(
        page.getByRole('heading', { name: 'Messora API' }).first(),
    ).toBeVisible();
    await expect(page.getByText('Authentication').first()).toBeVisible();

    // Search finds the messages operations.
    await page.getByRole('button', { name: /Open Search/ }).click();
    await page.keyboard.type('messages');
    const hit = page
        .getByRole('option')
        .filter({ hasText: 'Queue a WhatsApp message' });
    await expect(hit.first()).toBeVisible();
    await hit.first().click();

    // The operation shows a curl sample with the bearer key.
    const operation = page.locator('[id*="POST/messages"]').first();
    await expect(operation).toBeVisible();
    const sample = page
        .locator('code')
        .filter({ hasText: 'curl http' })
        .filter({ hasText: '/api/v1/messages ' });
    await expect(sample.first()).toBeVisible();
    await expect(sample.first()).toContainText(
        'Authorization: Bearer msk_live_YOUR_KEY',
    );

    expect(foreign, 'requests to other origins').toEqual([]);
    expect(problems, 'CSP violations or page errors').toEqual([]);
});
