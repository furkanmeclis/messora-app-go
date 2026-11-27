import { execFileSync } from 'node:child_process';
import { randomBytes } from 'node:crypto';
import { expect, test, type BrowserContext, type Page } from '@playwright/test';

/**
 * Workflow canvas: a manager builds a branched flow on the canvas (palette
 * clicks, a palette drag onto the canvas and a connection drawn between two
 * handles), saves it, reloads and finds the same graph; the steps stored for
 * the engine are the tree the canvas shows.
 *
 * Runs after identity.spec.ts (see playwright.config.ts). Like
 * contacts.spec.ts it signs in as a fresh manager invited straight through
 * the database.
 */

const PROJECT = process.env.E2E_PROJECT ?? 'messora-e2e';
const RUN = Date.now().toString(36);
const NAME = `Canvas ${RUN}`;
const MANAGER = {
    name: 'Cora Canvas',
    email: `cora+${RUN}@example.com`,
    password: 'Nodes-and-edges-41',
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
let automationUrl = '';

test.beforeAll(async ({ browser }) => {
    const token = `msi_${randomBytes(32).toString('base64url')}`;
    sql(`INSERT INTO user_invitations (email, role_id, token_hash, expires_at)
         SELECT '${MANAGER.email}', id, sha256(convert_to('${token}', 'UTF8')), now() + interval '1 hour'
         FROM roles WHERE key = 'manager';`);

    // A desktop-sized window: the canvas sits between the palette and the
    // inspector, and handles must be on screen to be dragged.
    context = await browser.newContext({
        viewport: { width: 1680, height: 1050 },
    });
    page = await context.newPage();
    page.on('pageerror', (e) => errors.push(e.message));
    page.on('console', (m) => {
        if (m.type() === 'error') errors.push(m.text());
    });
    await page.goto(`/invite?token=${token}`);
    await expect(page.getByText('You were invited as Manager.')).toBeVisible();
    await page.getByLabel('Your name').fill(MANAGER.name);
    await page.getByLabel('Password', { exact: true }).fill(MANAGER.password);
    await page.getByRole('button', { name: 'Join' }).click();
    await expect(
        page.getByRole('heading', { name: 'Welcome, Cora' }),
    ).toBeVisible();
});

test.afterAll(async () => {
    // Leave nothing that could react to later specs' events.
    sql(`DELETE FROM automations WHERE name = '${NAME}';`);
    await context?.close();
});

test.afterEach(() => {
    expect(errors, 'browser errors').toEqual([]);
});

const canvas = () => page.getByTestId('automation-canvas');
const inspector = () => page.getByTestId('inspector');
const node = (type: string) =>
    canvas().locator(`[data-testid="canvas-node"][data-step-type="${type}"]`);
const paletteItem = (type: string) =>
    canvas().locator(`[data-testid="palette-item"][data-step-type="${type}"]`);

/** Node ids and edges ("source:output→target") as the canvas shows them. */
async function graph(): Promise<{ nodes: string[]; edges: string[] }> {
    const nodes = await canvas()
        .locator('[data-testid="canvas-node"]')
        .evaluateAll((els) =>
            els
                .map(
                    (el) =>
                        `${(el as HTMLElement).dataset.nodeId}=${(el as HTMLElement).dataset.stepType}`,
                )
                .sort(),
        );
    const edges = await canvas()
        .locator('.react-flow__edge')
        .evaluateAll((els) =>
            els
                .map((el) => {
                    const id = el.getAttribute('data-id') ?? '';
                    return id;
                })
                .sort(),
        );
    return { nodes, edges };
}

/** Node id of the only node of a step type. */
async function idOf(type: string): Promise<string> {
    return (await node(type).getAttribute('data-node-id')) ?? '';
}

/** Selects a node by clicking its title area. */
async function select(type: string) {
    await node(type).locator('.font-semibold').first().click();
}

test('a manager builds a branched flow on the canvas', async () => {
    await page.goto('/automations/new');
    await expect(
        page.getByRole('heading', { name: 'New automation' }),
    ).toBeVisible();
    await expect(node('trigger')).toBeVisible();
    await expect(page.getByTestId('canvas-problems')).toBeVisible();

    await inspector().getByLabel('Name', { exact: true }).fill(NAME);

    // Nothing selected: a palette click appends to the end of the flow,
    // here right below the trigger.
    await paletteItem('condition').click();
    await expect(node('condition')).toBeVisible();
    const condition = await idOf('condition');
    // The new node is selected and its form is in the inspector.
    await expect(
        inspector().getByRole('heading', { name: 'Condition' }),
    ).toBeVisible();
    await inspector().getByRole('textbox', { name: 'Value' }).fill('vip');

    // With the condition selected, clicks fill its free outputs in order:
    // yes, then no.
    await paletteItem('reply').click();
    await expect(node('reply')).toBeVisible();
    await inspector()
        .getByRole('textbox', { name: 'Message' })
        .fill('Hello VIP {{first_name}}');

    await select('condition');
    await paletteItem('add_tag').click();
    await expect(node('add_tag')).toBeVisible();
    await inspector().getByRole('combobox', { name: 'Tag' }).fill('regular');

    // A node dragged from the palette lands unconnected …
    await paletteItem('delay').dragTo(canvas().locator('.react-flow__pane'), {
        targetPosition: { x: 120, y: 420 },
    });
    await expect(node('delay')).toBeVisible();
    await expect(node('delay')).toHaveAttribute('data-invalid', 'true');
    await expect(node('delay').getByTestId('node-error')).toContainText(
        'Not connected',
    );

    // … and is connected by drawing an edge from the condition's "then"
    // output (runs after either branch) to its input.
    const wait = await idOf('delay');
    await page.getByRole('button', { name: 'Fit to screen' }).click();
    await canvas()
        .locator(
            `.react-flow__node[data-id="${condition}"] .react-flow__handle[data-handleid="next"]`,
        )
        .dragTo(
            canvas().locator(
                `.react-flow__node[data-id="${wait}"] .react-flow__handle[data-handleid="in"]`,
            ),
        );
    await expect(
        canvas().locator(`.react-flow__edge[data-id="${condition}:next"]`),
    ).toHaveCount(1);
    await expect(node('delay')).not.toHaveAttribute('data-invalid');

    // A second incoming edge is refused (the graph stays a tree).
    const tag = await idOf('add_tag');
    await canvas()
        .locator(
            `.react-flow__node[data-id="${tag}"] .react-flow__handle[data-handleid="out"]`,
        )
        .dragTo(
            canvas().locator(
                `.react-flow__node[data-id="${wait}"] .react-flow__handle[data-handleid="in"]`,
            ),
        );
    await expect(
        canvas().locator(`.react-flow__edge[data-id="${tag}:out"]`),
    ).toHaveCount(0);

    // Undo and redo the tidy-up.
    await page.getByRole('button', { name: 'Tidy up the layout' }).click();
    await page.keyboard.press('ControlOrMeta+z');
    await page.keyboard.press('ControlOrMeta+Shift+z');

    await expect(canvas().locator('[data-invalid="true"]')).toHaveCount(0);
    const before = await graph();
    expect(before.nodes).toHaveLength(5);
    expect(before.edges.sort()).toEqual(
        [
            `trigger:out`,
            `${condition}:yes`,
            `${condition}:no`,
            `${condition}:next`,
        ].sort(),
    );

    await page.getByRole('button', { name: 'Create automation' }).click();
    await expect(page).toHaveURL(/\/automations\/[0-9a-f-]{36}$/);
    await expect(page.getByRole('heading', { name: NAME })).toBeVisible();
    automationUrl = page.url();

    // The engine gets the tree: the condition with then / else, then the wait.
    expect(
        sql(`SELECT concat_ws('|',
                    steps->0->>'type', steps->0->'if'->>'value',
                    steps->0->'then'->0->>'text', steps->0->'else'->0->>'tag',
                    steps->1->>'type', steps->1->>'id',
                    (SELECT count(*) FROM jsonb_object_keys(layout)))
             FROM automations WHERE name = '${NAME}'`),
    ).toBe(`condition|vip|Hello VIP {{first_name}}|regular|delay|${wait}|5`);

    // Reloading shows the same graph.
    await page.reload();
    await expect(node('trigger')).toBeVisible();
    await expect.poll(graph).toEqual(before);
    await expect(canvas().locator('[data-invalid="true"]')).toHaveCount(0);
});

test('moving a node is saved without a new version', async () => {
    await page.goto(automationUrl);
    await expect(node('delay')).toBeVisible();
    const version = sql(
        `SELECT version FROM automations WHERE name = '${NAME}'`,
    );
    const wait = await idOf('delay');
    const box = await node('delay').boundingBox();
    expect(box).not.toBeNull();
    await page.mouse.move(box!.x + 60, box!.y + 20);
    await page.mouse.down();
    await page.mouse.move(box!.x + 160, box!.y + 80, { steps: 8 });
    await page.mouse.up();

    await expect(page.getByTestId('redefine-warning')).toHaveCount(0);
    await page.getByRole('button', { name: 'Save changes' }).click();
    await expect(page.getByText('Automation saved.')).toBeVisible();
    expect(sql(`SELECT version FROM automations WHERE name = '${NAME}'`)).toBe(
        version,
    );
    const moved = sql(
        `SELECT layout->'${wait}'->>'x' FROM automations WHERE name = '${NAME}'`,
    );

    await page.reload();
    await expect(node('delay')).toBeVisible();
    expect(
        sql(
            `SELECT layout->'${wait}'->>'x' FROM automations WHERE name = '${NAME}'`,
        ),
    ).toBe(moved);
    const transform = await canvas()
        .locator(`.react-flow__node[data-id="${wait}"]`)
        .evaluate((el) => (el as HTMLElement).style.transform);
    expect(transform).toContain(`${moved}px`);
});
