'use client';

import { createContext, useCallback, useContext, useMemo } from 'react';
import {
    defaultPrefs,
    formatDate,
    formatDateTime,
    formatPrecise,
    formatTimeLabel,
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
    dateFormat,
    timeFormat,
    children,
}: {
    timeZone: string;
    locale: string;
    dateFormat?: string;
    timeFormat?: string;
    children: React.ReactNode;
}) {
    const value = useMemo(
        () => prefsFor(timeZone, locale, dateFormat, timeFormat),
        [timeZone, locale, dateFormat, timeFormat],
    );
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

/** formatPrecise bound to the user's display preferences. */
export function useFormatPrecise() {
    const prefs = useContext(Prefs);
    return useCallback(
        (iso: string | null | undefined) => formatPrecise(iso, prefs),
        [prefs],
    );
}

/** formatDate bound to the user's display preferences. */
export function useFormatDate() {
    const prefs = useContext(Prefs);
    return useCallback(
        (iso: string | null | undefined) => formatDate(iso, prefs),
        [prefs],
    );
}

/** The display preferences themselves (for custom formatting). */
export function useDisplayPrefs(): DisplayPrefs {
    return useContext(Prefs);
}

/** formatTimeLabel bound to the user's display preferences. */
export function useFormatTimeLabel() {
    const prefs = useContext(Prefs);
    return useCallback((iso: string) => formatTimeLabel(iso, prefs), [prefs]);
}
