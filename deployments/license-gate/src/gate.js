// Messora licence gate: a Cloudflare Worker.
//
//   POST /v1/check-in   installations report in; the answer is signed with
//                       the gate key ("msg1" envelope, see
//                       internal/platform/license).
//   GET  /v2/...        a read-only Docker registry over the R2 bucket.
//                       Username: anything; password: the licence key.
//                       Tags released after the licence's update window
//                       are refused.
//
// Bindings (wrangler.toml): KV (revocations, releases, sightings), R2
// (registry objects). Vars: LICENSE_KEYS = {"kid": "base64 public key"}.
// Secret: GATE_KEY = {"kid": "...", "seed": "base64 32-byte seed"}.

const enc = new TextEncoder();
const dec = new TextDecoder();

function b64urlToBytes(s) {
    s = s.replace(/-/g, '+').replace(/_/g, '/');
    while (s.length % 4) s += '=';
    return Uint8Array.from(atob(s), (c) => c.charCodeAt(0));
}

function bytesToB64url(b) {
    let s = '';
    for (const x of b) s += String.fromCharCode(x);
    return btoa(s).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

function b64ToBytes(s) {
    return Uint8Array.from(atob(s.trim()), (c) => c.charCodeAt(0));
}

const keyCache = new Map();

async function publicKey(b64) {
    if (!keyCache.has(b64)) {
        keyCache.set(
            b64,
            crypto.subtle.importKey('raw', b64ToBytes(b64), { name: 'Ed25519' }, false, ['verify']),
        );
    }
    return keyCache.get(b64);
}

// PKCS#8 wrapper for a raw Ed25519 seed.
const PKCS8_PREFIX = Uint8Array.from([
    0x30, 0x2e, 0x02, 0x01, 0x00, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x70, 0x04, 0x22, 0x04, 0x20,
]);

let gateKey;
async function signingKey(env) {
    if (!gateKey) {
        const { kid, seed } = JSON.parse(env.GATE_KEY);
        const raw = b64ToBytes(seed);
        if (raw.length !== 32) throw new Error('GATE_KEY seed must be 32 bytes');
        const der = new Uint8Array(PKCS8_PREFIX.length + 32);
        der.set(PKCS8_PREFIX);
        der.set(raw, PKCS8_PREFIX.length);
        gateKey = {
            kid,
            key: await crypto.subtle.importKey('pkcs8', der, { name: 'Ed25519' }, false, ['sign']),
        };
    }
    return gateKey;
}

/** Verifies an "msl1" licence key; returns its payload or null. */
export async function verifyLicense(token, env) {
    if (typeof token !== 'string' || token.length > 4096) return null;
    const parts = token.trim().split('.');
    if (parts.length !== 3 || parts[0] !== 'msl1') return null;
    let payload;
    try {
        payload = JSON.parse(dec.decode(b64urlToBytes(parts[1])));
    } catch {
        return null;
    }
    const keys = JSON.parse(env.LICENSE_KEYS || '{}');
    const pub = keys[payload?.kid];
    if (!pub || payload.v !== 1 || !payload.id) return null;
    let sig;
    try {
        sig = b64urlToBytes(parts[2]);
    } catch {
        return null;
    }
    const ok = await crypto.subtle.verify(
        { name: 'Ed25519' },
        await publicKey(pub),
        sig,
        enc.encode(parts[0] + '.' + parts[1]),
    );
    return ok ? payload : null;
}

async function sign(env, value) {
    const { kid, key } = await signingKey(env);
    const head = 'msg1.' + bytesToB64url(enc.encode(JSON.stringify({ ...value, v: 1, kid })));
    const sig = new Uint8Array(await crypto.subtle.sign({ name: 'Ed25519' }, key, enc.encode(head)));
    return head + '.' + bytesToB64url(sig);
}

function json(status, body, headers = {}) {
    return new Response(JSON.stringify(body), {
        status,
        headers: { 'content-type': 'application/json', ...headers },
    });
}

async function revocation(env, id) {
    const raw = await env.KV.get('revoked:' + id);
    return raw ? JSON.parse(raw) : null;
}

async function checkIn(request, env, ctx) {
    let body;
    try {
        body = await request.json();
    } catch {
        return json(400, { error: 'bad request' });
    }
    const instance = String(body?.instance_id || '').slice(0, 64);
    if (!/^[0-9a-f-]{36}$/.test(instance)) return json(400, { error: 'bad instance id' });
    const lic = await verifyLicense(body?.key, env);
    const answer = { license_id: lic?.id || '', instance_id: instance, at: new Date().toISOString() };
    if (!lic) {
        answer.status = 'unknown';
    } else {
        const rev = await revocation(env, lic.id);
        answer.status = rev ? rev.status : 'ok';
        const latest = await env.KV.get('release:latest:stable');
        if (latest) answer.latest = JSON.parse(latest);
        // Who runs what, for the issuer (kept 90 days after the last report).
        ctx.waitUntil(
            env.KV.put(
                `seen:${lic.id}:${instance}`,
                JSON.stringify({ at: answer.at, version: String(body?.version || '').slice(0, 32) }),
                { expirationTtl: 90 * 86400 },
            ),
        );
    }
    return new Response(await sign(env, answer), { headers: { 'content-type': 'text/plain' } });
}

// ---- registry --------------------------------------------------------------

const API = { 'Docker-Distribution-API-Version': 'registry/2.0' };
const REALM = { ...API, 'WWW-Authenticate': 'Basic realm="Messora registry"' };

function registryError(status, code, message) {
    return json(status, { errors: [{ code, message }] }, status === 401 ? REALM : API);
}

async function registryAuth(request, env) {
    const h = request.headers.get('authorization') || '';
    if (!h.startsWith('Basic ')) return { error: registryError(401, 'UNAUTHORIZED', 'Sign in with your licence key as the password.') };
    let password = '';
    try {
        const raw = atob(h.slice(6));
        password = raw.slice(raw.indexOf(':') + 1);
    } catch {
        return { error: registryError(401, 'UNAUTHORIZED', 'Malformed credentials.') };
    }
    const lic = await verifyLicense(password, env);
    if (!lic) return { error: registryError(401, 'UNAUTHORIZED', 'This licence key is not valid.') };
    const rev = await revocation(env, lic.id);
    if (rev && Date.now() - Date.parse(rev.at) > 7 * 86400e3) {
        return { error: registryError(403, 'DENIED', 'This licence was revoked.') };
    }
    if (lic.exp && Date.now() > Date.parse(lic.exp) + 14 * 86400e3) {
        return { error: registryError(403, 'DENIED', 'This licence has expired.') };
    }
    return { lic };
}

const NAME = /^[a-z0-9]+(?:[._-][a-z0-9]+)*(?:\/[a-z0-9]+(?:[._-][a-z0-9]+)*)*$/;
const DIGEST = /^sha256:[a-f0-9]{64}$/;
const TAG = /^[A-Za-z0-9_][A-Za-z0-9_.-]{0,127}$/;

async function registry(request, env, url) {
    const auth = await registryAuth(request, env);
    if (auth.error) return auth.error;
    const path = url.pathname.slice('/v2/'.length);
    if (path === '') return new Response('{}', { headers: { ...API, 'content-type': 'application/json' } });
    const m = path.match(/^(.+)\/(manifests|blobs)\/([^/]+)$/);
    if (!m || !NAME.test(m[1])) return registryError(404, 'NAME_UNKNOWN', 'Unknown repository.');
    const [, name, kind, ref] = m;
    if (kind === 'blobs' && !DIGEST.test(ref)) return registryError(404, 'BLOB_UNKNOWN', 'Unknown blob.');
    if (kind === 'manifests' && !DIGEST.test(ref)) {
        if (!TAG.test(ref)) return registryError(404, 'MANIFEST_UNKNOWN', 'Unknown tag.');
        // Updates window: the tag's release date must fall inside it.
        if (auth.lic.upd) {
            const rel = await env.KV.get('release:' + ref);
            const releasedAt = rel ? Date.parse(JSON.parse(rel).released_at) : NaN;
            if (!rel || releasedAt > Date.parse(auth.lic.upd)) {
                return registryError(403, 'DENIED', `Version ${ref} was released after your update period.`);
            }
        }
    }
    const obj = await (request.method === 'HEAD' ? env.R2.head : env.R2.get).call(
        env.R2,
        `v2/${name}/${kind}/${ref}`,
    );
    if (!obj) {
        return registryError(404, kind === 'blobs' ? 'BLOB_UNKNOWN' : 'MANIFEST_UNKNOWN', 'Not found.');
    }
    const headers = new Headers(API);
    headers.set('content-type', obj.httpMetadata?.contentType || 'application/octet-stream');
    headers.set('content-length', String(obj.size));
    let digest = DIGEST.test(ref) ? ref : obj.customMetadata?.digest;
    let body = obj.body;
    if (!digest && kind === 'manifests' && request.method !== 'HEAD') {
        // Manifests are small: hash the bytes (clients check this header).
        const bytes = new Uint8Array(await new Response(obj.body).arrayBuffer());
        digest = 'sha256:' + [...new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))].map((x) => x.toString(16).padStart(2, '0')).join('');
        body = bytes;
    } else if (!digest) {
        const full = await env.R2.get(`v2/${name}/${kind}/${ref}`);
        const bytes = new Uint8Array(await new Response(full.body).arrayBuffer());
        digest = 'sha256:' + [...new Uint8Array(await crypto.subtle.digest('SHA-256', bytes))].map((x) => x.toString(16).padStart(2, '0')).join('');
    }
    headers.set('docker-content-digest', digest);
    headers.set('etag', `"${digest}"`);
    return new Response(request.method === 'HEAD' ? null : body, { headers });
}

export default {
    async fetch(request, env, ctx) {
        const url = new URL(request.url);
        try {
            if (url.pathname === '/v1/check-in' && request.method === 'POST') {
                return await checkIn(request, env, ctx);
            }
            if ((url.pathname === '/v2/' || url.pathname.startsWith('/v2/')) && ['GET', 'HEAD'].includes(request.method)) {
                return await registry(request, env, url);
            }
            if (url.pathname === '/' || url.pathname === '/healthz') {
                return json(200, { ok: true, service: 'messora-license-gate' });
            }
            return json(404, { error: 'not found' });
        } catch (err) {
            console.error(err);
            return json(500, { error: 'internal error' });
        }
    },
};
