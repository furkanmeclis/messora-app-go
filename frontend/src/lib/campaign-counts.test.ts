import { test } from 'node:test';
import assert from 'node:assert/strict';
import { summarize } from './campaign-counts.ts';

test('summarize counts consent-skipped messages as skipped and done', () => {
    const s = summarize(
        {
            recipients: 10,
            recipients_skipped: 2,
            skipped: 1,
            sent: 3,
            delivered: 1,
            pending: 1,
            deferred: 2,
        },
        10,
    );
    assert.equal(s.skipped, 3);
    assert.equal(s.deferred, 2);
    // Deferred messages are neither in flight nor done.
    assert.equal(s.inFlight, 1);
    assert.equal(s.done, 7);
    assert.equal(s.percent, 70);
});

test('summarize adds recipient and message cancellations', () => {
    const s = summarize({ recipients_cancelled: 2, cancelled: 1 }, 4);
    assert.equal(s.cancelled, 3);
    assert.equal(s.done, 3);
    assert.equal(s.recipients, 4);
});

test('summarize never reports more than everyone done', () => {
    const s = summarize({ recipients: 2, sent: 5 }, 0);
    assert.equal(s.done, 2);
    assert.equal(s.percent, 100);
    assert.equal(summarize({}, 0).percent, 0);
});
