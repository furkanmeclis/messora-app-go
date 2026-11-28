import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
    billingBanner,
    formatIBAN,
    formatTL,
    fromKurus,
    isBillingError,
    toKurus,
    vatKurus,
    withVAT,
} from './billing.ts';

test('amounts convert between TL strings and kuruş', () => {
    assert.equal(toKurus('3588.00'), 358800);
    assert.equal(toKurus('2990'), 299000);
    assert.equal(toKurus('12.5'), 1250);
    assert.equal(toKurus('-0.05'), -5);
    assert.equal(toKurus('1,50'), 150);
    assert.equal(toKurus('abc'), null);
    assert.equal(toKurus('1.234'), null);
    assert.equal(fromKurus(358800), '3588.00');
    assert.equal(fromKurus(-5), '-0.05');
    assert.equal(fromKurus(7), '0.07');
});

test('VAT rounds half up to a kuruş', () => {
    assert.equal(vatKurus(299000, '20'), 59800);
    assert.equal(vatKurus(3, '20'), 1); // 0.6
    assert.equal(vatKurus(2, '20'), 0); // 0.4
    assert.equal(vatKurus(105, '10'), 11); // 10.5 → 11 (banker's: 10)
    assert.equal(vatKurus(125, '10'), 13); // 12.5 → 13 (banker's: 12)
    assert.equal(vatKurus(100, '0.5'), 1); // 0.5 → 1
    assert.deepEqual(withVAT('2990.00', '20'), {
        net: 299000,
        vat: 59800,
        total: 358800,
    });
    assert.equal(withVAT('x', '20'), null);
});

test('money and IBANs are formatted for people', () => {
    assert.equal(formatTL(358800), '3.588,00 TL');
    assert.equal(formatTL('1234567.89'), '1.234.567,89 TL');
    assert.equal(formatTL('0.05'), '0,05 TL');
    assert.equal(formatTL(null), '—');
    assert.equal(formatTL(358800, 'en'), 'TRY 3,588.00');
    assert.equal(
        formatIBAN('tr330006100519786457841326'),
        'TR33 0006 1005 1978 6457 8413 26',
    );
    assert.equal(
        formatIBAN('TR33 0006 1005 1978 6457 8413 26'),
        'TR33 0006 1005 1978 6457 8413 26',
    );
});

test('the most urgent billing banner wins', () => {
    const now = Date.parse('2026-11-28T12:00:00Z');
    assert.equal(billingBanner(null), null);
    assert.deepEqual(
        billingBanner({ read_only: true, subscription_status: 'read_only' }),
        { kind: 'read_only' },
    );
    assert.deepEqual(
        billingBanner({
            subscription_status: 'past_due',
            grace_ends_at: '2026-12-05T00:00:00Z',
        }),
        { kind: 'past_due', until: '2026-12-05T00:00:00Z' },
    );
    assert.deepEqual(
        billingBanner(
            {
                subscription_status: 'trial',
                trial_ends_at: '2026-11-30T12:00:00Z',
            },
            now,
        ),
        { kind: 'trial', until: '2026-11-30T12:00:00Z', days: 2 },
    );
    assert.equal(
        billingBanner(
            {
                subscription_status: 'trial',
                trial_ends_at: '2026-12-20T12:00:00Z',
            },
            now,
        ),
        null,
    );
    assert.deepEqual(
        billingBanner({
            subscription_status: 'active',
            message_balance_exhausted: true,
        }),
        { kind: 'balance' },
    );
    assert.equal(isBillingError('SUBSCRIPTION_READ_ONLY'), true);
    assert.equal(isBillingError('NOT_FOUND'), false);
});
