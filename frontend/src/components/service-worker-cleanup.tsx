'use client';

import { useEffect } from 'react';

/**
 * The installable app (PWA) was removed in Phase 17. Browsers that
 * installed its service worker keep it until it is unregistered, so every
 * page removes any registration and the worker's caches. Together with the
 * self-removing public/sw.js this can go after one release.
 */
export function ServiceWorkerCleanup() {
    useEffect(() => {
        if (!('serviceWorker' in navigator)) return;
        void navigator.serviceWorker
            .getRegistrations()
            .then((rs) => Promise.all(rs.map((r) => r.unregister())))
            .catch(() => {
                // not available (e.g. a private window): nothing to remove
            });
        if ('caches' in window) {
            void caches
                .keys()
                .then((keys) =>
                    Promise.all(
                        keys
                            .filter((k) => k.startsWith('messora-'))
                            .map((k) => caches.delete(k)),
                    ),
                )
                .catch(() => {});
        }
    }, []);
    return null;
}
