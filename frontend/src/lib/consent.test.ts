import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
    cleanKeywords,
    isConsentReason,
    isConsentStatus,
    refusedCount,
    sortReasons,
    statusTone,
} from './consent.ts';

test('statuses and reasons are recognised', () => {
    assert.ok(isConsentStatus('opt_out_requested'));
    assert.ok(!isConsentStatus('subscribed'));
    assert.ok(isConsentReason('meta_marketing_stopped'));
    assert.ok(!isConsentReason('duplicate'));
});

test('status tones', () => {
    assert.equal(statusTone('opted_in'), 'ok');
    assert.equal(statusTone('resubscribed'), 'ok');
    assert.equal(statusTone('unknown'), 'neutral');
    assert.equal(statusTone('opt_out_requested'), 'warn');
    assert.equal(statusTone('opted_out'), 'bad');
});

test('refusedCount counts only contacts who said no', () => {
    assert.equal(
        refusedCount([
            { reason: 'opted_out', count: 80 },
            { reason: 'meta_marketing_stopped', count: 3 },
            { reason: 'opt_out_requested', count: 2 },
            { reason: 'undeliverable', count: 7 },
            { reason: 'consent_unknown', count: 40 },
        ]),
        85,
    );
});

test('sortReasons puts hard blocks first, then by size', () => {
    const sorted = sortReasons([
        { reason: 'consent_unknown', count: 50, hard: false },
        { reason: 'undeliverable', count: 1, hard: true },
        { reason: 'opted_out', count: 9, hard: true },
    ]);
    assert.deepEqual(
        sorted.map((r) => r.reason),
        ['opted_out', 'undeliverable', 'consent_unknown'],
    );
});

test('cleanKeywords trims and drops blanks and repeats', () => {
    assert.deepEqual(cleanKeywords([' STOP ', 'stop', '', 'Dur', '  ']), [
        'STOP',
        'Dur',
    ]);
});
