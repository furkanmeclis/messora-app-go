import { defineConfig, devices } from '@playwright/test';

const device =
    devices[
        {
            webkit: 'Desktop Safari',
            firefox: 'Desktop Firefox',
        }[process.env.E2E_BROWSER ?? ''] ?? 'Desktop Chrome'
    ];

/**
 * End-to-end tests against a running Messora stack (see scripts/e2e.sh).
 * The base URL and one-time setup link come from the environment.
 */
export default defineConfig({
    testDir: './e2e',
    fullyParallel: false,
    workers: 1,
    forbidOnly: !!process.env.CI,
    retries: 0,
    timeout: 90_000,
    expect: { timeout: 15_000 },
    reporter: process.env.CI ? [['list'], ['html', { open: 'never' }]] : 'list',
    use: {
        baseURL: process.env.E2E_BASE_URL ?? 'http://localhost:8090',
        trace: 'retain-on-failure',
        screenshot: 'only-on-failure',
    },
    // CI runs Chromium; E2E_BROWSER=webkit|firefox runs another engine locally.
    // identity.spec.ts sets the installation up and creates the users the
    // other specs sign in with, so it always runs first (files would
    // otherwise run in alphabetical order).
    projects: [
        { name: 'identity', testMatch: 'identity.spec.ts', use: { ...device } },
        {
            name: process.env.E2E_BROWSER ?? 'chromium',
            testIgnore: 'identity.spec.ts',
            dependencies: ['identity'],
            use: { ...device },
        },
    ],
});
