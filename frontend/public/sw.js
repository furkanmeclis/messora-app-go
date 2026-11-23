/* Messora service worker: installable app shell and an offline page.
 * Panel data is never cached: only built, fingerprinted assets are
 * (cache-first); pages always come from the network and fall back to the
 * offline page when there is none. */
const VERSION = 'messora-v1';
const OFFLINE = '/offline.html';

self.addEventListener('install', (event) => {
    event.waitUntil(caches.open(VERSION).then((c) => c.add(OFFLINE)).then(() => self.skipWaiting()));
});

self.addEventListener('activate', (event) => {
    event.waitUntil(
        caches
            .keys()
            .then((keys) => Promise.all(keys.filter((k) => k !== VERSION).map((k) => caches.delete(k))))
            .then(() => self.clients.claim()),
    );
});

self.addEventListener('fetch', (event) => {
    const req = event.request;
    if (req.method !== 'GET') return;
    const url = new URL(req.url);
    if (url.origin !== self.location.origin) return;

    if (req.mode === 'navigate') {
        event.respondWith(fetch(req).catch(() => caches.match(OFFLINE)));
        return;
    }
    if (url.pathname.startsWith('/_next/static/')) {
        event.respondWith(
            caches.open(VERSION).then(async (cache) => {
                const hit = await cache.match(req);
                if (hit) return hit;
                const res = await fetch(req);
                if (res.ok) cache.put(req, res.clone());
                return res;
            }),
        );
    }
});
