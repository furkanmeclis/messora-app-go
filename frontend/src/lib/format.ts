/** How dates are shown: the signed-in user's zone and language. */
export type DisplayPrefs = { timeZone: string; locale: string };

/** Fixed fallback (also used before a user is known). */
export const defaultPrefs: DisplayPrefs = { timeZone: 'UTC', locale: 'en-GB' };

/** Display preferences from a profile; unknown zones fall back to UTC. */
export function prefsFor(timeZone: string, locale: string): DisplayPrefs {
    let tz = 'UTC';
    try {
        new Intl.DateTimeFormat('en', { timeZone }).format();
        tz = timeZone;
    } catch {
        // unknown zone: keep UTC
    }
    return { timeZone: tz, locale: locale === 'tr' ? 'tr-TR' : 'en-GB' };
}

/**
 * Date-time, e.g. "22 Nov 2026, 14:05". The zone and locale are explicit so
 * the server and the browser render the same text (no hydration mismatch).
 * Client components use useFormatDateTime() from display-preferences.
 */
export function formatDateTime(
    iso: string | null | undefined,
    prefs: DisplayPrefs = defaultPrefs,
): string {
    if (!iso) return '—';
    return new Intl.DateTimeFormat(prefs.locale, {
        dateStyle: 'medium',
        timeStyle: 'short',
        timeZone: prefs.timeZone,
    }).format(new Date(iso));
}

/** Date and time with seconds (step timelines are often seconds apart). */
export function formatPrecise(
    iso: string | null | undefined,
    prefs: DisplayPrefs = defaultPrefs,
): string {
    if (!iso) return '—';
    return new Intl.DateTimeFormat(prefs.locale, {
        dateStyle: 'medium',
        timeStyle: 'medium',
        timeZone: prefs.timeZone,
    }).format(new Date(iso));
}

/** Compact list label: the time for today, otherwise day and month. */
export function formatTimeLabel(
    iso: string,
    prefs: DisplayPrefs = defaultPrefs,
): string {
    const day = (d: Date) =>
        new Intl.DateTimeFormat('en-CA', { timeZone: prefs.timeZone }).format(
            d,
        );
    const d = new Date(iso);
    return day(d) === day(new Date())
        ? new Intl.DateTimeFormat(prefs.locale, {
              hour: '2-digit',
              minute: '2-digit',
              timeZone: prefs.timeZone,
          }).format(d)
        : new Intl.DateTimeFormat(prefs.locale, {
              day: 'numeric',
              month: 'short',
              timeZone: prefs.timeZone,
          }).format(d);
}

/** Numbers render the same on the server and in every browser. */
export const NUMBER_LOCALE = 'en-GB';

/** Rough, privacy-friendly browser/OS label from a user agent. */
export function describeUserAgent(ua: string | null | undefined): string {
    if (!ua) return 'Unknown device';
    const browser = /Edg\//.test(ua)
        ? 'Edge'
        : /Firefox\//.test(ua)
          ? 'Firefox'
          : /Chrome\//.test(ua)
            ? 'Chrome'
            : /Safari\//.test(ua)
              ? 'Safari'
              : 'Browser';
    const os = /Windows/.test(ua)
        ? 'Windows'
        : /Mac OS X/.test(ua)
          ? 'macOS'
          : /Android/.test(ua)
            ? 'Android'
            : /iPhone|iPad/.test(ua)
              ? 'iOS'
              : /Linux/.test(ua)
                ? 'Linux'
                : '';
    return os ? `${browser} on ${os}` : browser;
}
