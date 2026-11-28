import { test } from 'node:test';
import assert from 'node:assert/strict';
import { roleLabel } from './roles.ts';
import type { Translate } from './translate.ts';

const t = ((key: string) => `T:${key}`) as Translate;

test('built-in roles are translated', () => {
    assert.equal(
        roleLabel(t, { key: 'owner', name: 'Owner' }),
        'T:settings.security.roles.owner',
    );
    assert.equal(roleLabel(t, 'agent'), 'T:settings.security.roles.agent');
});

test('custom roles show their own name, then the key', () => {
    assert.equal(
        roleLabel(t, { key: 'e2e_meta', name: 'Meta (E2E)' }),
        'Meta (E2E)',
    );
    assert.equal(roleLabel(t, { key: 'support', name: '' }), 'support');
    assert.equal(roleLabel(t, 'support'), 'support');
});
