import { execFileSync } from 'node:child_process';
import { expect, test } from '@playwright/test';

/**
 * The panel's realtime connection through Caddy to Centrifugo: it connects
 * with tokens from the API (the CSP allows the same-origin WebSocket) and
 * recovers by itself after Centrifugo restarts. Runs after identity.spec.ts,
 * which creates the agent signing in here.
 */

const AGENT = { email: 'ari@example.com', password: 'Another-strong-pass-73' };
const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';

test('the panel reconnects after Centrifugo restarts', async ({ page }) => {
    const errors: string[] = [];
    page.on('pageerror', (e) => errors.push(e.message));
    page.on('console', (m) => {
        // The browser logs the dropped socket while Centrifugo is down.
        if (m.type() === 'error' && !/WebSocket/i.test(m.text())) {
            errors.push(m.text());
        }
    });

    await page.goto('/login');
    await page.getByLabel('E-mail').fill(AGENT.email);
    await page.getByLabel('Password').fill(AGENT.password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(
        page.getByRole('heading', { name: 'Welcome, Ari' }),
    ).toBeVisible();

    const state = page.getByTestId('realtime-state');
    await expect(state).toHaveText('Live');
    // Record every state the badge shows; reconnecting can be quick.
    await state.evaluate((el) => {
        const seen: string[] = [];
        (window as unknown as { rtSeen: string[] }).rtSeen = seen;
        new MutationObserver(() => seen.push(el.textContent ?? '')).observe(
            el,
            {
                childList: true,
                characterData: true,
                subtree: true,
            },
        );
    });

    execFileSync(
        'docker',
        ['compose', '-p', PROJECT, 'restart', 'centrifugo'],
        {
            cwd: '..',
            stdio: 'inherit',
        },
    );
    // No reload: the client notices the drop and reconnects with backoff.
    await expect
        .poll(
            () =>
                page.evaluate(
                    () => (window as unknown as { rtSeen: string[] }).rtSeen,
                ),
            {
                timeout: 30_000,
            },
        )
        .toContain('Connecting…');
    await expect(state).toHaveText('Live', { timeout: 60_000 });
    expect(errors, 'browser errors').toEqual([]);
});
