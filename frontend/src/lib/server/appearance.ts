import 'server-only';

import { cache } from 'react';
import { api } from './api';
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
// copy per web process keeps that from costing an API call (and an
// anonymous rate-limit slot) per page view.
const TTL_MS = 10_000;
let shared: { value: Appearance; expires: number } | null = null;

/** Forgets the shared copy (after the settings change in this process). */
export function invalidateAppearance() {
    shared = null;
}

/**
 * Look and regional defaults for this request (cached). A failing API must
 * not take the sign-in page down, so errors fall back to the defaults.
 */
export const getAppearance = cache(async (): Promise<Appearance> => {
    if (shared && shared.expires > Date.now()) return shared.value;
    try {
        const value = (await api<Appearance>('/appearance', { auth: 'none' }))
            .data;
        shared = { value, expires: Date.now() + TTL_MS };
        return value;
    } catch {
        return shared?.value ?? fallbackAppearance;
    }
});
