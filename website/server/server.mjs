// Messora website server: serves the static build and handles the forms.
// No dependencies; Node ≥ 22.
//
// Env: PORT (3000), DIST_DIR (../dist), RESEND_API_KEY, FORMS_TO,
//      FORMS_FROM ("Messora <no-reply@messora.tech>"), CANONICAL_HOST (messora.tech),
//      GTAG_ID, ADS_ID, ADS_LEAD_LABEL, ADS_SIGNUP_LABEL, GTAG_CONSENT_MODE (basic|advanced)
import { createServer } from 'node:http';
import { stat } from 'node:fs/promises';
import { createReadStream } from 'node:fs';
import { join, normalize, extname } from 'node:path';
import { randomBytes } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import { forms } from './forms-spec.mjs';
import { internalMail, confirmationMail } from './mail.mjs';

const here = fileURLToPath(new URL('.', import.meta.url));
const DIST = normalize(process.env.DIST_DIR || join(here, '../dist'));
const PORT = Number(process.env.PORT || 3000);
const HOST = process.env.CANONICAL_HOST || 'messora.tech';
const FROM = process.env.FORMS_FROM || 'Messora <no-reply@messora.tech>';

const log = (o) => process.stdout.write(JSON.stringify({ t: new Date().toISOString(), ...o }) + '\n');

const TYPES = {
  '.html': 'text/html; charset=utf-8', '.css': 'text/css; charset=utf-8', '.js': 'text/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8', '.webmanifest': 'application/manifest+json; charset=utf-8',
  '.xml': 'application/xml; charset=utf-8', '.txt': 'text/plain; charset=utf-8', '.svg': 'image/svg+xml',
  '.png': 'image/png', '.ico': 'image/x-icon', '.woff2': 'font/woff2', '.jpg': 'image/jpeg', '.webp': 'image/webp',
};

const G = 'https://*.googletagmanager.com https://www.googletagmanager.com';
const GA = 'https://*.google-analytics.com https://*.analytics.google.com https://*.g.doubleclick.net https://www.google.com https://*.google.com https://*.google.com.tr';
const CSP = [
  "default-src 'self'",
  `script-src 'self' 'unsafe-inline' ${G}`,
  "style-src 'self' 'unsafe-inline'",
  `img-src 'self' data: ${G} ${GA}`,
  "font-src 'self'",
  `connect-src 'self' ${G} ${GA}`,
  `frame-src ${G} https://td.doubleclick.net`,
  "base-uri 'self'", "form-action 'self'", "frame-ancestors 'none'", "object-src 'none'", 'upgrade-insecure-requests',
].join('; ');
const SEC = {
  'Strict-Transport-Security': 'max-age=31536000; includeSubDomains',
  'X-Content-Type-Options': 'nosniff',
  'Referrer-Policy': 'strict-origin-when-cross-origin',
  'X-Frame-Options': 'DENY',
  'Permissions-Policy': 'camera=(), microphone=(), geolocation=(), payment=()',
  'Content-Security-Policy': CSP,
};

// ── Static files ───────────────────────────────────────────
async function fileFor(pathname) {
  let p;
  try { p = decodeURIComponent(pathname); } catch { return null; }
  if (p.includes('\0')) return null;
  const candidates = extname(p) ? [p] : [join(p, 'index.html'), p + '.html'];
  for (const c of candidates) {
    const full = normalize(join(DIST, c));
    if (!full.startsWith(DIST)) return null;
    try { const s = await stat(full); if (s.isFile()) return { full, s }; } catch { /* next */ }
  }
  return null;
}

function cacheControl(pathname, ext) {
  if (pathname.startsWith('/_astro/') || pathname.startsWith('/fonts/')) return 'public, max-age=31536000, immutable';
  if (pathname.startsWith('/og/') || pathname.startsWith('/icons/') || ext === '.ico') return 'public, max-age=86400';
  if (ext === '.html' || ext === '') return 'public, max-age=0, must-revalidate';
  return 'public, max-age=3600';
}

async function serveFile(req, res, f, status = 200, pathname = '') {
  const ext = extname(f.full);
  const etag = `W/"${f.s.size.toString(36)}-${Math.floor(f.s.mtimeMs).toString(36)}"`;
  const headers = { ...SEC, 'Content-Type': TYPES[ext] || 'application/octet-stream', 'Cache-Control': status === 200 ? cacheControl(pathname, ext) : 'no-store', ETag: etag, Vary: 'Accept-Encoding' };
  if (status === 200 && req.headers['if-none-match'] === etag) { res.writeHead(304, headers); return res.end(); }
  const ae = String(req.headers['accept-encoding'] || '');
  let file = f.full;
  for (const [enc, suf] of [['br', '.br'], ['gzip', '.gz']]) {
    if (!ae.includes(enc)) continue;
    try { await stat(f.full + suf); file = f.full + suf; headers['Content-Encoding'] = enc; break; } catch { /* none */ }
  }
  res.writeHead(status, headers);
  if (req.method === 'HEAD') return res.end();
  createReadStream(file).pipe(res);
}

// ── Runtime config (Google tag) ───────────────────────────
function configJs() {
  const ok = (v, re) => (v && re.test(v) ? v : '');
  const cfg = {
    gtag: ok(process.env.GTAG_ID, /^G-[A-Z0-9]{4,20}$/),
    ads: ok(process.env.ADS_ID, /^AW-[0-9]{5,20}$/),
    leadLabel: ok(process.env.ADS_LEAD_LABEL, /^[A-Za-z0-9_-]{4,64}$/),
    signupLabel: ok(process.env.ADS_SIGNUP_LABEL, /^[A-Za-z0-9_-]{4,64}$/),
    mode: process.env.GTAG_CONSENT_MODE === 'advanced' ? 'advanced' : 'basic',
  };
  return `window.__MSR_CFG=${JSON.stringify(cfg)};\n`;
}

// ── Forms ─────────────────────────────────────────────────
const hits = new Map(); // ip -> timestamps
let globalHits = [];
function limited(ip) {
  const now = Date.now();
  const arr = (hits.get(ip) || []).filter((t) => now - t < 10 * 60_000);
  globalHits = globalHits.filter((t) => now - t < 60 * 60_000);
  if (arr.length >= 5 || globalHits.length >= 300) { hits.set(ip, arr); return true; }
  arr.push(now); globalHits.push(now); hits.set(ip, arr);
  return false;
}
setInterval(() => { const now = Date.now(); for (const [k, v] of hits) if (!v.some((t) => now - t < 10 * 60_000)) hits.delete(k); }, 60_000).unref();

const EMAIL = /^[^\s@<>"]{1,64}@[^\s@<>"]+\.[A-Za-z]{2,}$/;
const TEL = /^[+0-9 ()\-./]{7,40}$/;
const clean = (v, max) => String(v ?? '').replace(/[\u0000-\u0008\u000B\u000C\u000E-\u001F\u007F]/g, '').trim().slice(0, max);

function validate(spec, body) {
  const out = {};
  for (const f of spec.fields) {
    const raw = body[f.name];
    if (f.type === 'checkbox') {
      if (f.required && raw !== 'yes') return { field: f.name };
      out[f.name] = raw === 'yes';
      continue;
    }
    const v = clean(raw, f.max || 200);
    if (!v) { if (f.required) return { field: f.name }; out[f.name] = ''; continue; }
    if (f.type === 'email' && !EMAIL.test(v)) return { field: f.name };
    if (f.type === 'tel' && !TEL.test(v)) return { field: f.name };
    if (f.type === 'url' && !/^(https?:\/\/)?[^\s]+\.[^\s]+$/i.test(v)) return { field: f.name };
    if (f.type === 'select' && !f.options.some((o) => o.value === v)) return { field: f.name };
    if (f.type !== 'textarea' && /[\r\n]/.test(v)) return { field: f.name };
    out[f.name] = v;
  }
  return { data: out };
}

const CODE = { contact: 'C', demo: 'D', partner: 'P', lp: 'L', kvkk: 'K' };
function newRef(type) {
  const d = new Date();
  const ymd = `${String(d.getUTCFullYear()).slice(2)}${String(d.getUTCMonth() + 1).padStart(2, '0')}${String(d.getUTCDate()).padStart(2, '0')}`;
  const alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
  const rnd = [...randomBytes(5)].map((b) => alphabet[b % alphabet.length]).join('');
  return `MSR-${CODE[type]}${ymd}-${rnd}`;
}

async function resend(payload) {
  const key = process.env.RESEND_API_KEY;
  if (!key) throw new Error('RESEND_API_KEY missing');
  const res = await fetch('https://api.resend.com/emails', {
    method: 'POST',
    headers: { Authorization: `Bearer ${key}`, 'Content-Type': 'application/json' },
    body: JSON.stringify({ from: FROM, ...payload }),
    signal: AbortSignal.timeout(15_000),
  });
  if (!res.ok) throw new Error(`resend ${res.status}`);
  return res.json();
}

function json(res, status, obj) {
  res.writeHead(status, { ...SEC, 'Content-Type': 'application/json; charset=utf-8', 'Cache-Control': 'no-store' });
  res.end(JSON.stringify(obj));
}

async function readBody(req, limit = 32 * 1024) {
  let size = 0; const chunks = [];
  for await (const c of req) { size += c.length; if (size > limit) throw new Error('too large'); chunks.push(c); }
  return Buffer.concat(chunks).toString('utf8');
}

async function handleForm(req, res, type) {
  const spec = forms[type];
  if (!spec) return json(res, 404, { ok: false });
  const ct = String(req.headers['content-type'] || '');
  if (!ct.startsWith('application/json')) return json(res, 415, { ok: false });
  const origin = req.headers.origin;
  if (origin && !/^https:\/\/(www\.)?messora\.tech$/.test(origin) && !/^http:\/\/(localhost|127\.0\.0\.1)(:\d+)?$/.test(origin)) return json(res, 403, { ok: false });
  const ip = String(req.headers['x-real-ip'] || String(req.headers['x-forwarded-for'] || '').split(',')[0] || req.socket.remoteAddress || '').trim();
  if (limited(ip)) { log({ event: 'form', type, status: 'rate_limited' }); return json(res, 429, { ok: false }); }
  let body;
  try { body = JSON.parse(await readBody(req)); } catch { return json(res, 400, { ok: false }); }
  if (typeof body !== 'object' || !body) return json(res, 400, { ok: false });
  const ref = newRef(type);
  // Honeypot: pretend success, send nothing.
  if (body.hp_field) { log({ event: 'form', type, status: 'honeypot' }); return json(res, 200, { ok: true, ref }); }
  // Time trap: form must have been open for at least 3 s and at most 24 h.
  const age = Date.now() - Number(body.t);
  if (!Number.isFinite(age) || age < 3000 || age > 86_400_000) { log({ event: 'form', type, status: 'time_trap' }); return json(res, 400, { ok: false }); }
  const v = validate(spec, body);
  if (v.field) return json(res, 422, { ok: false, field: v.field });
  const lang = body.lang === 'en' ? 'en' : 'tr';
  const meta = {
    lang, page: clean(body.page, 200),
    attribution: Object.fromEntries(Object.entries(typeof body.attribution === 'object' && body.attribution ? body.attribution : {}).slice(0, 8).map(([k, val]) => [clean(k, 30), clean(val, 200)])),
  };
  try {
    const m = internalMail({ type, spec, ref, data: v.data, meta });
    await resend({ to: [process.env.FORMS_TO], reply_to: v.data.email, subject: m.subject, html: m.html, text: m.text });
  } catch (e) {
    log({ event: 'form', type, ref, status: 'send_failed', error: String(e.message || e).slice(0, 120) });
    return json(res, 502, { ok: false });
  }
  try {
    const c = confirmationMail({ type, spec, ref, data: v.data, lang });
    await resend({ to: [v.data.email], subject: c.subject, html: c.html, text: c.text });
  } catch (e) {
    log({ event: 'form', type, ref, status: 'confirmation_failed', error: String(e.message || e).slice(0, 120) });
  }
  log({ event: 'form', type, ref, status: 'sent', lang });
  return json(res, 200, { ok: true, ref });
}

// ── Router ────────────────────────────────────────────────
const server = createServer(async (req, res) => {
  try {
    const host = String(req.headers.host || '').toLowerCase().replace(/:\d+$/, '');
    const url = new URL(req.url || '/', 'http://x');
    if (host === `www.${HOST}`) {
      res.writeHead(301, { Location: `https://${HOST}${url.pathname}${url.search}`, 'Cache-Control': 'public, max-age=86400' });
      return res.end();
    }
    if (url.pathname === '/healthz') { res.writeHead(200, { 'Content-Type': 'text/plain' }); return res.end('ok'); }
    if (url.pathname === '/api/config.js') {
      res.writeHead(200, { ...SEC, 'Content-Type': 'text/javascript; charset=utf-8', 'Cache-Control': 'public, max-age=300' });
      return res.end(req.method === 'HEAD' ? undefined : configJs());
    }
    const fm = url.pathname.match(/^\/api\/forms\/([a-z]+)$/);
    if (fm) {
      if (req.method !== 'POST') { res.writeHead(405, { Allow: 'POST' }); return res.end(); }
      return await handleForm(req, res, fm[1]);
    }
    if (req.method !== 'GET' && req.method !== 'HEAD') { res.writeHead(405, { Allow: 'GET, HEAD' }); return res.end(); }
    if (url.pathname.length > 1 && url.pathname.endsWith('/')) {
      res.writeHead(301, { Location: url.pathname.replace(/\/+$/, '') + url.search });
      return res.end();
    }
    if (/\/index(\.html)?$/.test(url.pathname) || url.pathname.endsWith('.br') || url.pathname.endsWith('.gz')) {
      const f404 = await fileFor(url.pathname.startsWith('/en/') ? '/en/404' : '/404');
      return serveFile(req, res, f404, 404);
    }
    const f = await fileFor(url.pathname);
    if (f) return serveFile(req, res, f, 200, url.pathname);
    const f404 = await fileFor(url.pathname === '/en' || url.pathname.startsWith('/en/') ? '/en/404' : '/404');
    return serveFile(req, res, f404, 404);
  } catch (e) {
    log({ event: 'error', error: String(e?.message || e).slice(0, 200) });
    if (!res.headersSent) { res.writeHead(500, { 'Content-Type': 'text/plain' }); res.end('error'); }
  }
});
server.keepAliveTimeout = 65_000;
server.listen(PORT, () => log({ event: 'listen', port: PORT, dist: DIST, resend: !!process.env.RESEND_API_KEY, formsTo: !!process.env.FORMS_TO }));
for (const sig of ['SIGTERM', 'SIGINT']) process.on(sig, () => server.close(() => process.exit(0)));
