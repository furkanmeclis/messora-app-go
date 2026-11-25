import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
    ChannelPositions,
    EventDeduper,
    MAX_SYNC_GAP_MS,
    SYNC_MARGIN_MS,
    isInboxChannel,
    planResync,
} from './realtime-sync.ts';

test('deduper drops copies of an event and forgets the oldest IDs', () => {
    const d = new EventDeduper(3);
    assert.equal(d.seen('a'), false);
    assert.equal(d.seen('a'), true, 'second copy of a');
    assert.equal(d.seen('b'), false);
    assert.equal(d.seen('c'), false);
    assert.equal(d.seen('a'), true, 'a refreshed, now newest');
    assert.equal(d.seen('d'), false); // evicts b, the oldest
    assert.equal(d.size, 3);
    assert.equal(d.seen('b'), false, 'b was forgotten');
    assert.equal(d.seen('a'), true, 'a survived');
});

test('deduper never drops events without an ID', () => {
    const d = new EventDeduper();
    assert.equal(d.seen(undefined), false);
    assert.equal(d.seen(undefined), false);
    assert.equal(d.seen(''), false);
    assert.equal(d.seen(42), false);
    assert.equal(d.size, 0);
});

test('channel positions keep the newest moment and the last offset', () => {
    const p = new ChannelPositions();
    assert.equal(p.get('inbox:all'), undefined);
    p.mark('inbox:all', 1000, { offset: 5, epoch: 'e1' });
    p.mark('inbox:all', 900); // late, older: does not move back
    assert.deepEqual(p.get('inbox:all'), { offset: 5, epoch: 'e1', at: 1000 });
    p.mark('inbox:all', 2000, { offset: BigInt(7) });
    assert.deepEqual(p.get('inbox:all'), { offset: 7, epoch: 'e1', at: 2000 });
    p.mark('campaign:all', 10, { epoch: 'x' });
    assert.equal(p.get('campaign:all')?.epoch, 'x');
    assert.equal(p.get('inbox:all')?.epoch, 'e1');
});

test('resync plan: sync a short gap, reload a long or unknown one', () => {
    const now = Date.parse('2026-11-25T12:00:00Z');
    assert.deepEqual(planResync(undefined, now), { kind: 'reload' });
    assert.deepEqual(planResync({ at: now - MAX_SYNC_GAP_MS - 1 }, now), {
        kind: 'reload',
    });
    const at = now - 5 * 60 * 1000;
    assert.deepEqual(planResync({ at }, now), {
        kind: 'sync',
        since: new Date(at - SYNC_MARGIN_MS).toISOString(),
    });
});

test('inbox channels', () => {
    assert.ok(isInboxChannel('inbox:all'));
    assert.ok(isInboxChannel('inbox:unassigned'));
    assert.ok(isInboxChannel('personal:#0193a6c4-7f2e-7b3c-9d1e-4a5b6c7d8e9f'));
    assert.ok(!isInboxChannel('campaign:all'));
    assert.ok(!isInboxChannel('system:notifications'));
});
