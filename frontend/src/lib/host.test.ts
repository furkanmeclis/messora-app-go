import { test } from 'node:test';
import assert from 'node:assert/strict';
import { requestHost } from './host.ts';

const h = (host?: string) => new Headers(host === undefined ? {} : { host });

test('requestHost keeps a plain host and port, lowercased', () => {
    assert.equal(requestHost(h('Acme.Messora.tech')), 'acme.messora.tech');
    assert.equal(requestHost(h('localhost:8108')), 'localhost:8108');
});

test('requestHost refuses missing and malformed hosts', () => {
    assert.equal(requestHost(h()), null);
    assert.equal(requestHost(h('')), null);
    assert.equal(requestHost(h('evil.test/path')), null);
    assert.equal(requestHost(h('a b')), null);
    assert.equal(requestHost(h('a.test:99999999')), null);
});
