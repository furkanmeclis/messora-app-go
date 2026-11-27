import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
    configuredBaseDomain,
    isAdminHost,
    workspaceImageSource,
} from './platform-host.ts';

test('base domain from either variable', () => {
    assert.equal(
        configuredBaseDomain({ MESSORA_BASE_DOMAIN: ' Messora.Tech. ' }),
        'messora.tech',
    );
    assert.equal(
        configuredBaseDomain({ TENANT_BASE_DOMAIN: 'e2e.localhost' }),
        'e2e.localhost',
    );
    assert.equal(configuredBaseDomain({}), '');
});

test('admin host', () => {
    assert.ok(isAdminHost('admin.messora.tech', 'messora.tech'));
    assert.ok(isAdminHost('ADMIN.e2e.localhost:8090', 'e2e.localhost'));
    assert.ok(!isAdminHost('acme.messora.tech', 'messora.tech'));
    assert.ok(!isAdminHost('admin.messora.tech.evil.com', 'messora.tech'));
    assert.ok(!isAdminHost('admin.localhost', ''));
});

test('workspace logo source keeps the port', () => {
    assert.equal(
        workspaceImageSource('acme.messora.tech', true, 'messora.tech'),
        'https://*.messora.tech',
    );
    assert.equal(
        workspaceImageSource('localhost:8090', false, 'e2e.localhost'),
        'http://*.e2e.localhost:8090',
    );
    assert.equal(workspaceImageSource('x', true, 'bad domain'), '');
});
