/* The installable app (PWA) was removed in Phase 17. Browsers that still
 * have the old service worker fetch this file on their next update check:
 * it takes over, deletes the old caches, unregisters itself and reloads the
 * open pages, so they come straight from the network again. Keep it for
 * one release (see src/components/service-worker-cleanup.tsx). */
self.addEventListener('install', () => self.skipWaiting());

self.addEventListener('activate', (event) => {
    event.waitUntil(
        (async () => {
            const keys = await caches.keys();
            await Promise.all(keys.filter((k) => k.startsWith('messora-')).map((k) => caches.delete(k)));
            await self.registration.unregister();
            const pages = await self.clients.matchAll({ type: 'window' });
            pages.forEach((c) => c.navigate(c.url));
        })(),
    );
});
