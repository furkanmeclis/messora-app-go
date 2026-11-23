import { execFileSync } from 'node:child_process';
import { expect, test, type Page } from '@playwright/test';

/**
 * Automations: a manager builds a keyword automation with a condition
 * (then / else) and a delay, sees an API validation error next to the
 * offending step, turns it on and reads its run log. Runs after
 * identity.spec.ts (see the project dependency in playwright.config.ts),
 * which creates the agent; the agent is made a manager for this file only.
 *
 * Nothing is delivered in the E2E stack, so the run shown on the run log is
 * seeded in the database, as the worker would write it.
 */

const AGENT = { email: 'ari@example.com', password: 'Another-strong-pass-73' };
const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';

function sql(query: string): string {
    return execFileSync(
        'docker',
        [
            'compose', '-p', PROJECT, 'exec', '-T', 'postgres',
            'sh', '-c', 'psql -v ON_ERROR_STOP=1 -qtA -U "$POSTGRES_USER" -d "$POSTGRES_DB"',
        ],
        { cwd: '..', input: query, encoding: 'utf8' },
    ).trim();
}

function setRole(role: 'manager' | 'agent') {
    sql(`UPDATE user_roles SET role_id = (SELECT id FROM roles WHERE key = '${role}')
         WHERE user_id = (SELECT id FROM users WHERE email = '${AGENT.email}');`);
}

async function signIn(page: Page) {
    await page.goto('/login');
    await page.getByLabel('E-mail').fill(AGENT.email);
    await page.getByLabel('Password').fill(AGENT.password);
    await page.getByRole('button', { name: 'Sign in' }).click();
    await expect(page.getByRole('heading', { name: 'Welcome, Ari' })).toBeVisible();
}

async function choose(page: Page, combobox: ReturnType<Page['getByRole']>, option: string) {
    await combobox.click();
    await page.getByRole('option', { name: option, exact: true }).click();
}

test.describe.configure({ mode: 'serial' });

test.beforeAll(() => setRole('manager'));
test.afterAll(() => {
    setRole('agent');
    // Leave nothing that could react to later tests' events.
    sql(`UPDATE automations SET active = false;`);
});

test('a manager builds an automation with a condition and a delay', async ({ page }) => {
    const errors: string[] = [];
    page.on('pageerror', (e) => errors.push(e.message));

    await signIn(page);
    await page.getByRole('link', { name: 'Automations', exact: true }).click();
    await expect(page.getByRole('heading', { name: 'Automations', exact: true })).toBeVisible();
    await expect(page.getByText('No automations yet')).toBeVisible();
    await page.getByRole('link', { name: 'New automation' }).click();
    await expect(page.getByRole('heading', { name: 'New automation' })).toBeVisible();

    await page.getByLabel('Name', { exact: true }).fill('Price question');

    // Trigger: a message containing one of two keywords.
    await choose(page, page.getByRole('combobox', { name: 'Message' }), 'Contains a keyword');
    const keywords = page.getByRole('textbox', { name: 'Keywords' });
    await keywords.fill('price');
    await keywords.press('Enter');
    await keywords.fill('fiyat');
    await keywords.press('Enter');
    await expect(page.getByTestId('trigger-summary')).toHaveText(
        "When a message contains 'price' or 'fiyat'",
    );

    // Step 1: a condition with a reply (then) and a wait (else).
    await page.getByRole('button', { name: 'Add step', exact: true }).click();
    await page.getByRole('menuitem', { name: /^Condition/ }).click();
    const condition = page.locator('[data-testid="step-card"][data-step-type="condition"]');
    await expect(condition).toBeVisible();
    await condition.getByRole('textbox', { name: 'Value' }).fill('urgent');

    const then = condition.getByTestId('branch-then');
    await then.getByRole('button', { name: 'Add step to then' }).click();
    await page.getByRole('menuitem', { name: 'Reply' }).click();
    const reply = then.getByRole('textbox', { name: 'Message' });
    await reply.fill('Hi ');
    // Placeholder chips insert at the cursor.
    await then.getByRole('button', { name: 'Insert placeholder into Message' }).click();
    await page.getByRole('menuitem', { name: '{{first_name}}' }).click();
    await expect(reply).toHaveValue('Hi {{first_name}}');
    await reply.press('End');
    await reply.pressSequentially(', our price list is on its way.');

    const otherwise = condition.getByTestId('branch-else');
    await otherwise.getByRole('button', { name: 'Add step to else' }).click();
    await page.getByRole('menuitem', { name: 'Wait' }).click();
    await otherwise.getByRole('spinbutton', { name: 'Wait for' }).fill('2');
    await choose(page, otherwise.getByRole('combobox', { name: 'Wait for unit' }), 'hours');

    // Step 2: tag the contact.
    await page.getByRole('button', { name: 'Add step', exact: true }).click();
    await page.getByRole('menuitem', { name: 'Add tag' }).click();
    await page
        .locator('[data-testid="step-card"][data-step-type="add_tag"]')
        .getByRole('textbox', { name: 'Tag' })
        .fill('price-asked');

    await page.getByRole('button', { name: 'Create automation' }).click();
    await expect(page).toHaveURL(/\/automations\/[0-9a-f-]{36}$/);
    await expect(page.getByRole('heading', { name: 'Price question' })).toBeVisible();
    expect(
        sql(`SELECT concat_ws('|', active, steps->0->'then'->0->>'text', steps->0->'else'->0->>'seconds', steps->1->>'tag')
             FROM automations WHERE name = 'Price question'`),
    ).toBe('f|Hi {{first_name}}, our price list is on its way.|7200|price-asked');

    // An invalid value is reported next to the step that holds it.
    await page.getByRole('button', { name: 'Add step', exact: true }).click();
    await page.getByRole('menuitem', { name: 'HTTP request' }).click();
    const http = page.locator('[data-testid="step-card"][data-step-type="http_request"]');
    await http.getByRole('textbox', { name: 'URL' }).fill('http://example.com/hook');
    await expect(page.getByTestId('redefine-warning')).toBeVisible();
    await page.getByRole('button', { name: 'Save changes' }).click();
    await expect(http).toHaveAttribute('data-invalid', 'true');
    await expect(http.getByText('Use an https:// URL.')).toBeVisible();
    await expect(http.getByRole('textbox', { name: 'URL' })).toHaveAttribute('aria-invalid', 'true');
    await expect(condition).not.toHaveAttribute('data-invalid');

    // Removing the step makes the automation valid again.
    await http.getByRole('button', { name: 'Remove Step 3' }).click();
    await expect(page.getByTestId('redefine-warning')).toHaveCount(0);

    expect(errors, 'browser errors').toEqual([]);
});

test('the list turns it on and the run log shows each executed step', async ({ page }) => {
    // Seeded before the list is opened: GET /automations currently fails
    // for an automation that has never run (last_run_at NULL is scanned
    // into a non-nullable time; see the report on the automations branch).
    sql(`
        WITH c AS (INSERT INTO contacts (phone_e164, name) VALUES ('+905551112233', 'Run Test') RETURNING id),
             a AS (SELECT id, version FROM automations WHERE name = 'Price question'),
             r AS (INSERT INTO automation_runs (automation_id, automation_version, event_key, event, contact_id,
                                                status, pc, steps_executed, finished_at)
                   SELECT a.id, a.version, 'e2e-run-1',
                          '{"type": "message_received", "body": "urgent price?"}'::jsonb, c.id,
                          'completed', 4, 3, now()
                   FROM a, c RETURNING id)
        INSERT INTO automation_run_steps (run_id, pc, action, outcome, detail)
        SELECT r.id, s.pc, s.action, s.outcome, s.detail::jsonb
        FROM r, (VALUES (0, 'branch', 'ok', '{"result": true}'),
                        (1, 'reply', 'skipped', '{"reason": "WINDOW_CLOSED", "message": "The 24-hour window is closed."}'),
                        (4, 'add_tag', 'ok', '{}')) AS s (pc, action, outcome, detail);`);

    await signIn(page);
    await page.getByRole('link', { name: 'Automations', exact: true }).click();
    const row = page.getByTestId('automation-row').filter({ hasText: 'Price question' });
    await expect(row.getByRole('cell').nth(3)).toHaveText('1');

    // Turn it on from the list; the trigger reads as plain language.
    await expect(row).toContainText("When a message contains 'price' or 'fiyat'");
    const toggle = row.getByRole('switch', { name: 'Active: Price question' });
    await expect(toggle).not.toBeChecked();
    await toggle.click();
    await expect(toggle).toBeChecked();
    await expect.poll(() => sql(`SELECT active FROM automations WHERE name = 'Price question'`)).toBe('t');

    await row.getByRole('link', { name: 'Runs of Price question' }).click();
    await expect(page.getByRole('heading', { name: 'Runs · Price question' })).toBeVisible();

    const run = page.getByTestId('run-row').first();
    await expect(run).toContainText('+905551112233');
    await expect(run).toContainText('Completed');
    await run.getByRole('button', { name: /^Details of the run/ }).click();

    const drawer = page.getByRole('dialog');
    await expect(drawer.getByRole('heading', { name: 'Run details' })).toBeVisible();
    const steps = drawer.getByTestId('run-step');
    await expect(steps).toHaveCount(3);
    await expect(steps.nth(0)).toContainText('Condition was true: continued with the then steps.');
    await expect(steps.nth(1)).toContainText('Skipped');
    await expect(steps.nth(1)).toContainText('The 24-hour window is closed.');
    await expect(steps.nth(2)).toContainText('Add tag');
    await expect(drawer).toContainText('urgent price?');
    await page.keyboard.press('Escape');
    await expect(drawer).toBeHidden();
});
