import { test } from 'node:test';
import assert from 'node:assert/strict';
import { sessionCache } from './session-cache.ts';

function counter(values: string[] = ['v1', 'v2', 'v3']) {
    let calls = 0;
    return {
        load: async () => values[calls++] ?? 'more',
        get calls() {
            return calls;
        },
    };
}

test('a value is loaded once within the TTL and again after it', async () => {
    let t = 0;
    const c = sessionCache<string>(5_000, 10, () => t);
    const l = counter();
    assert.equal(await c.get('a', l.load), 'v1');
    t = 4_999;
    assert.equal(await c.get('a', l.load), 'v1');
    assert.equal(l.calls, 1);
    t = 5_000;
    assert.equal(await c.get('a', l.load), 'v2');
    assert.equal(l.calls, 2);
});

test('sessions do not share entries', async () => {
    const c = sessionCache<string>(5_000, 10);
    assert.equal(await c.get('a', async () => 'alice'), 'alice');
    assert.equal(await c.get('b', async () => 'bob'), 'bob');
    assert.equal(await c.get('a', async () => 'other'), 'alice');
});

test('concurrent loads share one request', async () => {
    const c = sessionCache<string>(5_000, 10);
    const l = counter();
    const got = await Promise.all([
        c.get('a', l.load),
        c.get('a', l.load),
        c.get('a', l.load),
    ]);
    assert.deepEqual(got, ['v1', 'v1', 'v1']);
    assert.equal(l.calls, 1);
});

test('errors are never cached', async () => {
    const c = sessionCache<string>(5_000, 10);
    let n = 0;
    const flaky = async () => {
        n++;
        if (n === 1) throw new Error('429');
        return 'ok';
    };
    await assert.rejects(c.get('a', flaky), /429/);
    assert.equal(await c.get('a', flaky), 'ok');
    assert.equal(n, 2);
});

test('invalidate drops the entry and a load racing with it is not stored', async () => {
    const c = sessionCache<string>(5_000, 10);
    assert.equal(await c.get('a', async () => 'old'), 'old');
    c.invalidate('a');
    let release!: (v: string) => void;
    const racing = c.get('a', () => new Promise<string>((r) => (release = r)));
    c.invalidate('a'); // the change lands while the read is in flight
    release('stale');
    assert.equal(await racing, 'stale');
    assert.equal(await c.get('a', async () => 'fresh'), 'fresh');
});

test('the map is bounded', async () => {
    const c = sessionCache<string>(5_000, 3);
    for (const k of ['a', 'b', 'c', 'd']) await c.get(k, async () => k);
    assert.ok(c.size <= 3);
});
