import { test } from 'node:test';
import assert from 'node:assert/strict';
import gate, { verifyLicense } from '../src/gate.js';

const enc = new TextEncoder();
const b64url = (b) => Buffer.from(b).toString('base64url');

async function keypair() {
    const k = await crypto.subtle.generateKey({ name: 'Ed25519' }, true, ['sign', 'verify']);
    const pub = Buffer.from(await crypto.subtle.exportKey('raw', k.publicKey)).toString('base64');
    const pkcs8 = new Uint8Array(await crypto.subtle.exportKey('pkcs8', k.privateKey));
    return { k, pub, seed: Buffer.from(pkcs8.slice(-32)).toString('base64') };
}

async function issue(priv, payload) {
    const head = 'msl1.' + b64url(enc.encode(JSON.stringify(payload)));
    const sig = await crypto.subtle.sign({ name: 'Ed25519' }, priv, enc.encode(head));
    return head + '.' + b64url(new Uint8Array(sig));
}

function memoryEnv(lic, gk) {
    const kv = new Map();
    const r2 = new Map();
    return {
        kv,
        r2,
        LICENSE_KEYS: JSON.stringify({ k1: lic.pub }),
        GATE_KEY: JSON.stringify({ kid: 'g1', seed: gk.seed }),
        KV: {
            get: async (k) => kv.get(k) ?? null,
            put: async (k, v) => void kv.set(k, v),
        },
        R2: {
            get: async (k) => r2.get(k) ?? null,
            head: async (k) => r2.get(k) ?? null,
        },
    };
}

const ctx = { waitUntil: (p) => p };
// A well-formed but unsigned licence.
const FORGED = ['msl1', 'forged', 'signature'].join('.');
const instance = '0b0c1f0e-3c3a-4f5e-9d6f-1a2b3c4d5e6f';

async function openAnswer(text, pub) {
    const [p, payload, sig] = text.split('.');
    assert.equal(p, 'msg1');
    const key = await crypto.subtle.importKey('raw', Buffer.from(pub, 'base64'), { name: 'Ed25519' }, false, ['verify']);
    assert.ok(
        await crypto.subtle.verify({ name: 'Ed25519' }, key, Buffer.from(sig, 'base64url'), enc.encode(p + '.' + payload)),
        'gate signature',
    );
    return JSON.parse(Buffer.from(payload, 'base64url').toString());
}

test('check-in answers are signed and carry revocations', async () => {
    const lic = await keypair();
    const gk = await keypair();
    const env = memoryEnv(lic, gk);
    const key = await issue(lic.k.privateKey, { v: 1, id: 'lic_1', kid: 'k1', customer: 'Acme', iat: '2026-01-01T00:00:00Z', chk: 'online' });
    assert.ok(await verifyLicense(key, env));
    assert.equal(await verifyLicense(key.slice(0, -4) + 'AAAA', env), null);

    env.kv.set('release:latest:stable', JSON.stringify({ version: '1.0.0', released_at: '2026-11-01T00:00:00Z', channel: 'stable' }));
    const call = (body) =>
        gate.fetch(new Request('https://gate/v1/check-in', { method: 'POST', body: JSON.stringify(body) }), env, ctx);
    let a = await openAnswer(await (await call({ key, instance_id: instance, version: '0.9.0' })).text(), gk.pub);
    assert.equal(a.status, 'ok');
    assert.equal(a.kid, 'g1');
    assert.equal(a.instance_id, instance);
    assert.equal(a.latest.version, '1.0.0');
    assert.ok(env.kv.has(`seen:lic_1:${instance}`));

    env.kv.set('revoked:lic_1', JSON.stringify({ status: 'revoked', at: new Date().toISOString() }));
    a = await openAnswer(await (await call({ key, instance_id: instance })).text(), gk.pub);
    assert.equal(a.status, 'revoked');

    a = await openAnswer(await (await call({ key: FORGED, instance_id: instance })).text(), gk.pub);
    assert.equal(a.status, 'unknown');
    assert.equal((await call({ key, instance_id: 'x' })).status, 400);
});

test('registry needs a licence and honours the update window', async () => {
    const lic = await keypair();
    const gk = await keypair();
    const env = memoryEnv(lic, gk);
    const manifest = '{"schemaVersion":2}';
    const obj = (body, type) => ({ body: new TextEncoder().encode(body), size: body.length, httpMetadata: { contentType: type } });
    env.r2.set('v2/messora/manifests/1.0.0', obj(manifest, 'application/vnd.oci.image.index.v1+json'));
    env.r2.set('v2/messora/manifests/2.0.0', obj(manifest, 'application/vnd.oci.image.index.v1+json'));
    env.kv.set('release:1.0.0', JSON.stringify({ released_at: '2026-06-01T00:00:00Z' }));
    env.kv.set('release:2.0.0', JSON.stringify({ released_at: '2027-06-01T00:00:00Z' }));
    const key = await issue(lic.k.privateKey, { v: 1, id: 'lic_2', kid: 'k1', iat: '2026-01-01T00:00:00Z', chk: 'offline', upd: '2026-12-31T23:59:59Z' });
    const get = (path, password) =>
        gate.fetch(
            new Request('https://gate' + path, {
                headers: password ? { authorization: 'Basic ' + btoa('x:' + password) } : {},
            }),
            env,
            ctx,
        );
    let r = await get('/v2/');
    assert.equal(r.status, 401);
    assert.match(r.headers.get('www-authenticate'), /Basic/);
    assert.equal((await get('/v2/', 'msl1.bad.key')).status, 401);
    assert.equal((await get('/v2/', key)).status, 200);
    r = await get('/v2/messora/manifests/1.0.0', key);
    assert.equal(r.status, 200);
    const want = 'sha256:' + Buffer.from(await crypto.subtle.digest('SHA-256', enc.encode(manifest))).toString('hex');
    assert.equal(r.headers.get('docker-content-digest'), want);
    assert.equal(await r.text(), manifest);
    assert.equal((await get('/v2/messora/manifests/2.0.0', key)).status, 403);
    assert.equal((await get('/v2/messora/manifests/9.9.9', key)).status, 403);
    assert.equal((await get('/v2/messora/blobs/sha256:' + 'b'.repeat(64), key)).status, 404);
    assert.equal((await get('/v2/../secret/manifests/1.0.0', key)).status, 404);
});

test('install file needs a licence', async () => {
    const lic = await keypair();
    const gk = await keypair();
    const env = memoryEnv(lic, gk);
    const file = 'name: messora\n';
    env.r2.set('install/stable/compose.yml', { body: new TextEncoder().encode(file), size: file.length });
    env.r2.set('install/2.0.0/compose.yml', { body: new TextEncoder().encode(file), size: file.length });
    env.kv.set('release:2.0.0', JSON.stringify({ released_at: '2027-06-01T00:00:00Z' }));
    const key = await issue(lic.k.privateKey, { v: 1, id: 'lic_3', kid: 'k1', iat: '2026-01-01T00:00:00Z', chk: 'offline', upd: '2026-12-31T23:59:59Z' });
    const get = (path, password) =>
        gate.fetch(new Request('https://gate' + path, { headers: password ? { authorization: 'Basic ' + btoa('x:' + password) } : {} }), env, ctx);
    let r = await get('/install/latest/compose.yml');
    assert.equal(r.status, 401);
    assert.match(r.headers.get('www-authenticate'), /Basic/);
    r = await get('/install/latest/compose.yml', key);
    assert.equal(r.status, 200);
    assert.equal(await r.text(), file);
    assert.equal((await get('/install/2.0.0/compose.yml', key)).status, 403);
    assert.equal((await get('/install/../secret/compose.yml', key)).status, 404);
});
