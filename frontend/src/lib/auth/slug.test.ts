import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
    RESERVED_SLUGS,
    sanitizeSlugInput,
    slugify,
    validateSlug,
} from './slug.ts';
import { tenantSlugFromHost } from './tenant-host.ts';

test('valid workspace addresses pass', () => {
    for (const s of ['acme', 'easy-trade', 'a1b', 'x'.repeat(30), '123']) {
        assert.equal(validateSlug(s), null, s);
    }
});

test('each rule reports its own error', () => {
    assert.equal(validateSlug(''), 'required');
    assert.equal(validateSlug('ab'), 'too_short');
    assert.equal(validateSlug('x'.repeat(31)), 'too_long');
    assert.equal(validateSlug('Acme'), 'invalid_chars');
    assert.equal(validateSlug('acme_co'), 'invalid_chars');
    assert.equal(validateSlug('çay'), 'invalid_chars');
    assert.equal(validateSlug('-acme'), 'edge_hyphen');
    assert.equal(validateSlug('acme-'), 'edge_hyphen');
    assert.equal(validateSlug('xn--acme'), 'double_hyphen');
});

test('every platform sub-domain is reserved', () => {
    for (const s of [
        'www',
        'admin',
        'api',
        'app',
        'hooks',
        'dokploy',
        'cdn',
        'mail',
        'docs',
        'status',
        'help',
        'support',
    ]) {
        assert.ok(RESERVED_SLUGS.includes(s), s);
        // "api" is only three letters; the reserved check must still win.
        assert.equal(validateSlug(s), 'reserved', s);
    }
    assert.equal(validateSlug('apis'), null);
});

test('typing is cleaned but a trailing hyphen survives', () => {
    assert.equal(sanitizeSlugInput('Easy Trade'), 'easy-trade');
    assert.equal(sanitizeSlugInput('ŞEKER_çiçek'), 'seker-cicek');
    assert.equal(sanitizeSlugInput('İstanbul'), 'istanbul');
    assert.equal(sanitizeSlugInput('acme-'), 'acme-');
    assert.equal(sanitizeSlugInput('a@b!c'), 'abc');
    assert.equal(sanitizeSlugInput('x'.repeat(40)).length, 30);
});

test('company names become a valid suggestion', () => {
    assert.equal(slugify('Easy Trade A.Ş.'), 'easy-trade-a-s');
    assert.equal(slugify('  Gül & Kahve  '), 'gul-kahve');
    assert.equal(slugify('Café Noël'), 'cafe-noel');
    const long = slugify('Çok Uzun Bir Şirket Adı Limited Şirketi');
    assert.ok(long.length <= 30);
    assert.equal(validateSlug(long), null);
});

test('tenant comes from a workspace sub-domain only', () => {
    const base = 'messora.tech';
    assert.equal(tenantSlugFromHost('acme.messora.tech', base), 'acme');
    assert.equal(tenantSlugFromHost('ACME.messora.tech:443', base), 'acme');
    assert.equal(tenantSlugFromHost('messora.tech', base), null);
    assert.equal(tenantSlugFromHost('app.messora.tech', base), null);
    assert.equal(tenantSlugFromHost('a.b.messora.tech', base), null);
    assert.equal(tenantSlugFromHost('evilmessora.tech', base), null);
    assert.equal(tenantSlugFromHost('localhost:3000', base), null);
    assert.equal(tenantSlugFromHost(null, base), null);
});
