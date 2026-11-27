import 'server-only';

import { cache } from 'react';
import { headers } from 'next/headers';
import { api } from './api';
import { requestHost } from './tenant-host';
import type { Appearance } from '@/lib/types';

export const fallbackAppearance: Appearance = {
    organization_name: 'Messora',
    default_locale: 'en',
    default_timezone: 'UTC',
    date_format: 'dd.MM.yyyy',
    time_format: '24h',
    logo: null,
    setup_completed: true,
};

// Every page needs the appearance, the sign-in page included. One shared
// copy per company address and web process keeps that from costing an API
// call (and an anonymous rate-limit slot) per page view.
const TTL_MS = 10_000;
const MAX_HOSTS = 1_000;
const shared = new Map<string, { value: Appearance; expires: number }>();

/** Forgets the shared copies (after the settings change in this process). */
export function invalidateAppearance() {
    shared.clear();
}

/**
 * Look and regional defaults for this request (cached). A failing API must
 * not take the sign-in page down, so errors fall back to the defaults.
 */
export const getAppearance = cache(async (): Promise<Appearance> => {
    const host = requestHost(await headers()) ?? '';
    const hit = shared.get(host);
    if (hit && hit.expires > Date.now()) return hit.value;
    try {
        const value = (await api<Appearance>('/appearance', { auth: 'none' }))
            .data;
        if (shared.size >= MAX_HOSTS) shared.clear();
        shared.set(host, { value, expires: Date.now() + TTL_MS });
        return value;
    } catch {
        return hit?.value ?? fallbackAppearance;
    }
});
