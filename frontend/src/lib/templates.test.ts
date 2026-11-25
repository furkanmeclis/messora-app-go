import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
    emptyValues,
    fieldOf,
    issuesOf,
    paramsFromValues,
    resolvePieces,
    shapeOf,
    valuesFromParams,
} from './templates.ts';
import type { Template } from './types';

const tpl = (components: unknown[], format = 'POSITIONAL') =>
    ({ components, parameter_format: format }) as unknown as Template;

const photo = tpl([
    {
        type: 'HEADER',
        format: 'IMAGE',
        example: { header_handle: ['https://x/h.jpg'] },
    },
    {
        type: 'BODY',
        text: 'Hi {{2}} and {{1}}',
        example: { body_text: [['Ayşe', 'A-1']] },
    },
    {
        type: 'BUTTONS',
        buttons: [
            { type: 'QUICK_REPLY', text: 'Stop' },
            {
                type: 'URL',
                text: 'Go',
                url: 'https://s/{{1}}',
                example: ['https://s/x'],
            },
        ],
    },
]);

test('shape of a media template', () => {
    const s = shapeOf(photo);
    assert.equal(s.header, 'image');
    assert.deepEqual(s.bodyVars, ['1', '2']);
    assert.deepEqual(s.urlButtons, [1]);
    assert.equal(s.exampleMedia, 'https://x/h.jpg');
    assert.equal(s.examples['body:1'], 'Ayşe');
});

test('params round trip with sparse buttons', () => {
    const s = shapeOf(photo);
    const v = emptyValues();
    v.body = { '1': '{{first_name}}', '2': 'Summer' };
    v.buttons = { 1: '{{var1}}' };
    v.media = { type: 'image', link: 'https://cdn/p.jpg' };
    v.fallback = { first_name: 'friend', email: 'unused' };
    const p = paramsFromValues(v, s);
    assert.deepEqual(p.body, ['{{first_name}}', 'Summer']);
    assert.deepEqual(p.buttons, ['', '{{var1}}']);
    assert.deepEqual(p.fallback, { first_name: 'friend' });
    assert.deepEqual(valuesFromParams(p, s).buttons, { 1: '{{var1}}' });
    assert.deepEqual(issuesOf(v, s), []);
    assert.deepEqual(
        issuesOf(emptyValues(), s).map((i) =>
            i.kind === 'value' ? i.slot : i.kind,
        ),
        ['media', 'body:1', 'body:2', 'button:1'],
    );
});

test('named templates use objects', () => {
    const s = shapeOf(
        tpl([{ type: 'BODY', text: 'Code {{code}} for {{name}}' }], 'NAMED'),
    );
    const v = emptyValues();
    v.body = { code: '42', name: '{{name}}' };
    assert.deepEqual(paramsFromValues(v, s).body, {
        code: '42',
        name: '{{name}}',
    });
});

test('placeholders resolve against a sample contact', () => {
    assert.equal(fieldOf(' {{attr:city}} '), 'attr:city');
    assert.equal(fieldOf('Hi {{name}}'), undefined);
    const pieces = resolvePieces(
        'Hi {{first_name}} from {{attr:city}} {{var2}}',
        { name: 'Ayşe Yılmaz', attributes: { city: 'İzmir' } },
        { var2: '!' },
    );
    assert.deepEqual(pieces, [
        { text: 'Hi ' },
        { field: 'first_name', value: 'Ayşe' },
        { text: ' from ' },
        { field: 'attr:city', value: 'İzmir' },
        { text: ' ' },
        { field: 'var2', value: '!', fallback: true },
    ]);
});

const lto = tpl([
    { type: 'HEADER', format: 'IMAGE', example: { header_handle: ['h'] } },
    {
        type: 'LIMITED_TIME_OFFER',
        limited_time_offer: { text: 'Hurry', has_expiration: true },
    },
    { type: 'BODY', text: 'Hi {{1}}', example: { body_text: [['A']] } },
    {
        type: 'BUTTONS',
        buttons: [
            { type: 'COPY_CODE', example: 'SAVE' },
            {
                type: 'URL',
                text: 'Go',
                url: 'https://s/{{1}}',
                example: ['https://s/x'],
            },
        ],
    },
]);

test('limited-time offer: copy code, URL index shift and offer fills', () => {
    const s = shapeOf(lto);
    assert.equal(s.lto, true);
    assert.deepEqual(s.copyCode, [0]);
    assert.deepEqual(s.urlButtons, [1]);
    const v = emptyValues();
    v.body = { '1': '{{first_name}}' };
    v.buttons = { 1: 'x' };
    v.media = { type: 'image', link: 'https://c/i.jpg' };
    // Without an offer the code and the end are needed.
    assert.deepEqual(
        issuesOf(v, s).map((i) => (i.kind === 'value' ? i.slot : i.kind)),
        ['coupon', 'expiration'],
    );
    assert.deepEqual(issuesOf(v, s, { coupon: true, expiration: true }), []);
    v.coupon = 'SAVE10';
    v.expirationMs = 1700000000000;
    const p = paramsFromValues(v, s);
    assert.equal(p.coupon_code, 'SAVE10');
    assert.equal(p.expiration_time_ms, 1700000000000);
    assert.deepEqual(p.buttons, ['', 'x']);
});

test('carousel cards, location, OTP and Flow params', () => {
    const carousel = shapeOf(
        tpl([
            { type: 'BODY', text: 'Look' },
            {
                type: 'CAROUSEL',
                cards: [0, 1].map(() => ({
                    components: [
                        { type: 'HEADER', format: 'VIDEO' },
                        { type: 'BODY', text: 'Card {{1}}' },
                        {
                            type: 'BUTTONS',
                            buttons: [
                                { type: 'QUICK_REPLY', text: 'More' },
                                {
                                    type: 'URL',
                                    text: 'Shop',
                                    url: 'https://s/{{1}}',
                                },
                            ],
                        },
                    ],
                })),
            },
        ]),
    );
    assert.equal(carousel.cards.length, 2);
    assert.equal(carousel.cards[0]!.header, 'video');
    assert.deepEqual(carousel.cards[0]!.quickReplies, [0]);
    const v = emptyValues();
    v.cards = [
        {
            body: { '1': 'a' },
            buttons: { 0: 'more-a', 1: 'a' },
            media: { type: 'video', link: 'https://v/a.mp4' },
        },
        { body: {}, buttons: {} },
    ];
    const kinds = issuesOf(v, carousel).map((i) =>
        i.kind === 'value' ? i.slot : `${i.kind}`,
    );
    assert.deepEqual(kinds, ['cardMedia', 'card:1:body:1', 'card:1:button:1']);
    const p = paramsFromValues(v, carousel);
    assert.deepEqual(p.cards?.[0], {
        header_media: { type: 'video', link: 'https://v/a.mp4' },
        body: ['a'],
        buttons: ['more-a', 'a'],
    });
    assert.deepEqual(p.cards?.[1], { body: [''] });

    const loc = shapeOf(tpl([{ type: 'HEADER', format: 'LOCATION' }]));
    const lv = emptyValues();
    assert.deepEqual(issuesOf(lv, loc), [{ kind: 'location' }]);
    lv.location = {
        latitude: '41.01',
        longitude: '28.97',
        name: 'Mağaza',
        address: '',
    };
    assert.deepEqual(issuesOf(lv, loc), []);
    assert.deepEqual(paramsFromValues(lv, loc).location, {
        latitude: '41.01',
        longitude: '28.97',
        name: 'Mağaza',
    });

    // Meta returns the OTP button as a URL button: the category decides.
    const otp = shapeOf({
        ...tpl([
            { type: 'BODY', text: '{{1}} is your verification code.' },
            {
                type: 'BUTTONS',
                buttons: [
                    {
                        type: 'URL',
                        text: 'Copy',
                        url: 'https://www.whatsapp.com/otp/code/?code=otp{{1}}',
                    },
                ],
            },
        ]),
        category: 'AUTHENTICATION',
    } as Template);
    assert.deepEqual(otp.otp, [0]);
    assert.deepEqual(otp.urlButtons, []);
    const ov = emptyValues();
    assert.deepEqual(issuesOf(ov, otp), [{ kind: 'value', slot: 'otp' }]);
    ov.otp = '123456';
    assert.deepEqual(paramsFromValues(ov, otp), { otp_code: '123456' });

    const flow = shapeOf(
        tpl([
            { type: 'BODY', text: 'Book' },
            {
                type: 'BUTTONS',
                buttons: [{ type: 'FLOW', text: 'Book', flow_id: '1' }],
            },
        ]),
    );
    const fv = emptyValues();
    fv.flowData = '{"a": 1}';
    assert.deepEqual(paramsFromValues(fv, flow).flow, {
        flow_action_data: { a: 1 },
    });
    fv.flowData = '[1]';
    assert.deepEqual(issuesOf(fv, flow), [{ kind: 'flowData' }]);
});
