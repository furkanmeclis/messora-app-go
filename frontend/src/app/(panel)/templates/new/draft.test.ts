import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
    buildComponents,
    emptyCard,
    emptyDraft,
    nextVariable,
    validateDraft,
    type Draft,
} from './draft.ts';

// Messages are not needed: keys stand in for them.
const t = ((key: string) => key) as unknown as Parameters<
    typeof validateDraft
>[0];

function draft(patch: Partial<Draft>): Draft {
    return { ...emptyDraft('acc'), name: 'x', ...patch };
}

test('limited-time offer: Meta documented shape', () => {
    const d = draft({
        kind: 'lto',
        headerKind: 'IMAGE',
        headerHandle: '4::aW...',
        ltoText: 'Expiring offer!',
        ltoExpiration: true,
        body: 'Good news, {{1}}! Use code {{2}}.',
        examples: { 'b:1': 'Pablo', 'b:2': 'CARIBE25' },
        ltoCoupon: 'CARIBE25',
        ltoUrl: {
            text: 'Book now!',
            value: 'https://a.test/offers?code={{1}}',
            example: 'https://a.test/offers?ref=n3mtql',
        },
    });
    assert.deepEqual(validateDraft(t, d), {});
    assert.deepEqual(buildComponents(d), [
        {
            type: 'HEADER',
            format: 'IMAGE',
            example: { header_handle: ['4::aW...'] },
        },
        {
            type: 'LIMITED_TIME_OFFER',
            limited_time_offer: {
                text: 'Expiring offer!',
                has_expiration: true,
            },
        },
        {
            type: 'BODY',
            text: 'Good news, {{1}}! Use code {{2}}.',
            example: { body_text: [['Pablo', 'CARIBE25']] },
        },
        {
            type: 'BUTTONS',
            buttons: [
                { type: 'COPY_CODE', example: 'CARIBE25' },
                {
                    type: 'URL',
                    text: 'Book now!',
                    url: 'https://a.test/offers?code={{1}}',
                    example: ['https://a.test/offers?ref=n3mtql'],
                },
            ],
        },
    ]);
    // Meta's limits: offer text 16, coupon 15, body 600.
    const bad = validateDraft(t, {
        ...d,
        ltoText: 'x'.repeat(17),
        ltoCoupon: 'x'.repeat(16),
        body: 'x'.repeat(601),
    });
    assert.ok(bad['lto-text'] && bad['lto-coupon'] && bad.body);
});

test('carousel cards share one structure', () => {
    const card = (k: number, handle: string, slug: string) => ({
        ...emptyCard(k, ['QUICK_REPLY', 'URL']),
        handle,
        buttons: [
            { text: 'More', value: '', example: '' },
            {
                text: 'Shop',
                value: 'https://s.test/{{1}}',
                example: slug,
            },
        ],
    });
    const d = draft({
        kind: 'carousel',
        body: 'Rare succulents, {{1}}!',
        examples: { 'b:1': 'Pablo' },
        cardButtons: ['QUICK_REPLY', 'URL'],
        cards: [card(1, 'h1', 'BLUE'), card(2, 'h2', 'RED')],
    });
    assert.deepEqual(validateDraft(t, d), {});
    const comps = buildComponents(d);
    assert.equal(comps.length, 2);
    const carousel = comps[1] as {
        type: string;
        cards: { components: { type: string }[] }[];
    };
    assert.equal(carousel.type, 'CAROUSEL');
    assert.equal(carousel.cards.length, 2);
    assert.deepEqual(
        carousel.cards[1]!.components.map((c) => c.type),
        ['HEADER', 'BUTTONS'],
    );
    // One card or a missing sample is refused.
    const one = validateDraft(t, { ...d, cards: [d.cards[0]!] });
    assert.ok(one.cards);
    const noSample = validateDraft(t, {
        ...d,
        cards: [d.cards[0]!, { ...d.cards[1]!, handle: '' }],
    });
    assert.ok(noSample['card-1-media']);
});

test('authentication: OTP button variants', () => {
    const d = draft({
        kind: 'authentication',
        category: 'AUTHENTICATION',
        auth: {
            otpType: 'ONE_TAP',
            security: true,
            expiry: '5',
            text: 'Copy',
            autofill: 'Autofill',
            packageName: 'com.example.app',
            signatureHash: 'K8a/AINcGX7',
            zeroTapTerms: false,
        },
    });
    assert.deepEqual(validateDraft(t, d), {});
    assert.deepEqual(buildComponents(d), [
        { type: 'BODY', add_security_recommendation: true },
        { type: 'FOOTER', code_expiration_minutes: 5 },
        {
            type: 'BUTTONS',
            buttons: [
                {
                    type: 'OTP',
                    otp_type: 'ONE_TAP',
                    text: 'Copy',
                    autofill_text: 'Autofill',
                    supported_apps: [
                        {
                            package_name: 'com.example.app',
                            signature_hash: 'K8a/AINcGX7',
                        },
                    ],
                },
            ],
        },
    ]);
    const zero = validateDraft(t, {
        ...d,
        auth: { ...d.auth, otpType: 'ZERO_TAP', signatureHash: 'short' },
    });
    assert.ok(zero['otp-terms'] && zero['otp-hash']);
    assert.ok(
        validateDraft(t, { ...d, auth: { ...d.auth, expiry: '91' } })[
            'otp-expiry'
        ],
    );
});

test('standard buttons: limits and grouping', () => {
    const b = (type: Draft['buttons'][0]['type'], value = '') => ({
        key: Math.random(),
        type,
        text: 'Go',
        value,
        example: '',
        flowAction: 'navigate' as const,
    });
    const ok = draft({
        body: 'Hi',
        buttons: [
            b('QUICK_REPLY'),
            b('QUICK_REPLY'),
            b('URL', 'https://a.test/'),
            b('COPY_CODE', 'SAVE10'),
            b('FLOW', '1234'),
        ],
    });
    assert.deepEqual(validateDraft(t, ok), {});
    const flow = (buildComponents(ok)[1] as { buttons: unknown[] }).buttons[4];
    assert.deepEqual(flow, {
        type: 'FLOW',
        text: 'Go',
        flow_id: '1234',
        flow_action: 'navigate',
    });
    const split = draft({
        body: 'Hi',
        buttons: [
            b('QUICK_REPLY'),
            b('URL', 'https://a.test/'),
            b('QUICK_REPLY'),
        ],
    });
    assert.ok(validateDraft(t, split).buttons);
    const url = draft({
        body: 'Hi',
        buttons: [b('URL', 'https://a.test/{{1}}/x')],
    });
    assert.ok(validateDraft(t, url)['button-0-value']);
});

test('next variable', () => {
    assert.equal(nextVariable('Hi {{1}} {{2}}', false), '3');
    assert.equal(nextVariable('', false), '1');
    assert.equal(nextVariable('Hi {{name}}', true), 'value_2');
});
