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
