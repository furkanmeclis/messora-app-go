// Live plan data for the static pages. The build renders every plan block
// from a snapshot (dist/plans-snapshot.json). This module keeps the live API
// response in memory (5 min, stale-while-revalidate, ETag) and, when it
// differs from the snapshot, re-renders the plan blocks of the affected pages.
// Requests never wait for the API: until the first fetch succeeds, or when
// the API is down or returns an unexpected shape, the snapshot is used.
import { readFile, readdir } from 'node:fs/promises';
import { join } from 'node:path';
import { gzipSync, brotliCompressSync, constants } from 'node:zlib';
import { normalize, hashPlans, rerender, phrases, PLANS_API_URL } from './plans-core.mjs';

const TTL = 5 * 60_000;
const RETRY = 60_000;

export function createPlans({ dist, log, url = process.env.PLANS_API_URL || PLANS_API_URL }) {
  let snapshot = null, snapHash = '';
  let live = null, liveHash = '', etag = '', fetchedAt = 0, inflight = null;
  const planPages = new Set();
  const cache = new Map(); // file -> { hash, body, gz, br, etag }
  const contact = (l) => (l === 'tr' ? '/iletisim' : '/en/contact');

  async function init() {
    try {
      snapshot = normalize(JSON.parse(await readFile(join(dist, 'plans-snapshot.json'), 'utf8')));
      snapHash = snapshot ? hashPlans(snapshot) : '';
    } catch (e) { log({ event: 'plans_snapshot_missing', err: e.message }); }
    if (!snapshot) return;
    const marks = ['data-ps="', 'data-plans-ld="', ...['tr', 'en'].flatMap((l) => Object.values(phrases(snapshot, l)).filter(Boolean))];
    const walk = async (d) => {
      for (const e of await readdir(d, { withFileTypes: true })) {
        const p = join(d, e.name);
        if (e.isDirectory()) await walk(p);
        else if (e.name.endsWith('.html')) {
          const html = await readFile(p, 'utf8');
          if (marks.some((m) => html.includes(m))) planPages.add(p);
        }
      }
    };
    await walk(dist);
    log({ event: 'plans_init', pages: planPages.size, snapshot: snapHash, api: url });
    refresh();
  }

  function refresh() {
    if (inflight || !url || url === 'off') return inflight;
    inflight = (async () => {
      try {
        const res = await fetch(url, { signal: AbortSignal.timeout(5000), headers: { Accept: 'application/json', ...(etag ? { 'If-None-Match': etag } : {}) } });
        if (res.status === 304 && live) { fetchedAt = Date.now(); return; }
        if (!res.ok) throw new Error(`status ${res.status}`);
        const plans = normalize(await res.json());
        if (!plans) throw new Error('unexpected shape');
        const h = hashPlans(plans);
        if (h !== liveHash) { log({ event: 'plans_updated', hash: h, sameAsBuild: h === snapHash }); cache.clear(); }
        live = plans; liveHash = h; etag = res.headers.get('etag') || ''; fetchedAt = Date.now();
      } catch (e) {
        log({ event: 'plans_fetch_failed', err: e.message });
        fetchedAt = Date.now() - TTL + RETRY; // keep what we have, try again in a minute
      } finally { inflight = null; }
    })();
    return inflight;
  }

  /** Returns a re-rendered page body, or null when the static file is current. */
  async function page(full) {
    if (!snapshot || !planPages.has(full)) return null;
    if (Date.now() - fetchedAt > TTL) refresh();
    if (!live || liveHash === snapHash) return null;
    const hit = cache.get(full);
    if (hit && hit.hash === liveHash) return hit;
    try {
      const html = rerender(await readFile(full, 'utf8'), snapshot, live, contact);
      const body = Buffer.from(html);
      const entry = {
        hash: liveHash, body, etag: `W/"p-${liveHash}-${body.length.toString(36)}"`,
        gz: gzipSync(body, { level: 6 }),
        br: brotliCompressSync(body, { params: { [constants.BROTLI_PARAM_QUALITY]: 6 } }),
      };
      cache.set(full, entry);
      return entry;
    } catch (e) {
      log({ event: 'plans_render_failed', err: e.message });
      return null;
    }
  }

  return { init, page, refresh, state: () => ({ snapshot: snapHash, live: liveHash || null, fetchedAt }) };
}
