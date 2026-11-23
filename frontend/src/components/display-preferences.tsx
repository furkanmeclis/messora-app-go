'use client';

import { createContext, useCallback, useContext, useMemo } from 'react';
import {
    defaultPrefs,
    formatDateTime,
    prefsFor,
    type DisplayPrefs,
} from '@/lib/format';

const Prefs = createContext<DisplayPrefs>(defaultPrefs);

/**
 * Provides the signed-in user's time zone and language. The values come from
 * the server, so server rendering and hydration produce the same text.
 */
export function DisplayPreferences({
    timeZone,
    locale,
    children,
}: {
    timeZone: string;
    locale: string;
    children: React.ReactNode;
}) {
    const value = useMemo(() => prefsFor(timeZone, locale), [timeZone, locale]);
    return <Prefs.Provider value={value}>{children}</Prefs.Provider>;
}

/** formatDateTime bound to the user's display preferences. */
export function useFormatDateTime() {
    const prefs = useContext(Prefs);
    return useCallback(
        (iso: string | null | undefined) => formatDateTime(iso, prefs),
        [prefs],
    );
}
