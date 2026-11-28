import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
    formatBytes,
    formatCredits,
    formatPrice,
    hasFeature,
    parseOverrides,
    planError,
    usagePercent,
    usageTone,
} from './plan.ts';

test('hasFeature treats a missing plan as everything included', () => {
    assert.equal(hasFeature({}, 'custom_smtp'), true);
    assert.equal(hasFeature({ tenant: {} }, 'byo_ai_keys'), true);
    const me = { tenant: { plan: { features: ['webhooks', 'ai_agents'] } } };
    assert.equal(hasFeature(me, 'webhooks'), true);
    assert.equal(hasFeature(me, 'custom_smtp'), false);
    assert.equal(hasFeature(me, 'public_api'), false);
});

test('usage percent and tone', () => {
    assert.equal(usagePercent(1, 4), 25);
    assert.equal(usagePercent(9, 4), 100);
    assert.equal(usagePercent(5, 0), 0);
    assert.equal(usageTone(5, 0), 'unlimited');
    assert.equal(usageTone(1, 5), 'ok');
    assert.equal(usageTone(4, 5), 'high');
    assert.equal(usageTone(5, 5), 'high');
    assert.equal(usageTone(6, 5), 'over');
});

test('formatBytes', () => {
    assert.equal(formatBytes(0), '0 B');
    assert.equal(formatBytes(512), '512 B');
    assert.equal(formatBytes(1536), '1.5 KB');
    assert.equal(formatBytes(5 * 1024 ** 3), '5 GB');
    assert.equal(formatBytes(150 * 1024 ** 2), '150 MB');
    assert.equal(formatBytes(1536, 'tr'), '1,5 KB');
});

test('formatCredits and formatPrice', () => {
    assert.equal(formatCredits(12.3456), '12.346');
    assert.equal(formatCredits(2000), '2,000');
    assert.equal(formatPrice(null, 'TRY'), null);
    assert.equal(formatPrice('abc', 'TRY'), null);
    assert.match(formatPrice('1290', 'TRY', 'tr') ?? '', /1\.290/);
    assert.match(formatPrice('12.5', 'USD', 'en') ?? '', /12\.50/);
});

test('planError reads limit and feature details', () => {
    assert.deepEqual(
        planError('PLAN_LIMIT_REACHED', [
            { field: 'limit', code: 'users', message: '5' },
            { field: 'current', code: 'users', message: '5' },
        ]),
        { kind: 'limit', resource: 'users', limit: 5, current: 5 },
    );
    assert.deepEqual(
        planError('PLAN_LIMIT_REACHED', [
            { field: 'limit', code: 'webhook_endpoints', message: '3' },
        ]),
        {
            kind: 'limit',
            resource: 'webhook_endpoints',
            limit: 3,
            current: null,
        },
    );
    assert.deepEqual(
        planError('FEATURE_NOT_IN_PLAN', [
            { field: 'feature', code: 'custom_smtp' },
        ]),
        { kind: 'feature', feature: 'custom_smtp' },
    );
    assert.equal(planError('VALIDATION_FAILED', []), null);
    assert.equal(planError(undefined, null), null);
});

test('parseOverrides validates keys and values', () => {
    assert.deepEqual(parseOverrides(''), { ok: true, value: {} });
    assert.deepEqual(
        parseOverrides('{"users": 50, "features": {"custom_smtp": true}}'),
        {
            ok: true,
            value: { users: 50, features: { custom_smtp: true } },
        },
    );
    assert.equal(parseOverrides('[1]').ok, false);
    assert.equal(parseOverrides('{').ok, false);
    assert.deepEqual(parseOverrides('{"seats": 3}'), {
        ok: false,
        error: 'unknown:seats',
    });
    assert.deepEqual(parseOverrides('{"users": -1}'), {
        ok: false,
        error: 'users',
    });
    assert.deepEqual(parseOverrides('{"features": {"x": true}}'), {
        ok: false,
        error: 'unknown:x',
    });
    assert.deepEqual(parseOverrides('{"features": {"webhooks": 1}}'), {
        ok: false,
        error: 'features.webhooks',
    });
});
