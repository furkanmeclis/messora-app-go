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
    theme: {
        preset: 'messora',
        mode: 'system',
        light: {},
        dark: {},
        radius: 0.75,
        font: 'geist',
        density: 'comfortable',
        wallpaper: 'aurora',
    },
    logo: null,
    setup_completed: true,
};

/**
 * Look and regional defaults for this request (cached). A failing API must
 * not take the sign-in page down, so errors fall back to the defaults.
 */
export const getAppearance = cache(async (): Promise<Appearance> => {
    try {
        return (await api<Appearance>('/appearance', { auth: 'none' })).data;
    } catch {
        return fallbackAppearance;
    }
});
