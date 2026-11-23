// Dates must render identically on the server and during hydration, so the
// default is fixed (UTC, British English). After hydration the panel
// switches to the signed-in user's time zone and language — see
// DisplayPreferences in components/display-preferences.tsx.
let prefs = { timeZone: 'UTC', locale: 'en-GB' };

/** Applies the user's display preferences (browser only). */
export function setDisplayPreferences(timeZone: string, locale: string) {
    let tz = 'UTC';
    try {
        new Intl.DateTimeFormat('en', { timeZone }).format();
        tz = timeZone;
    } catch {
        // unknown zone: keep UTC
    }
    prefs = { timeZone: tz, locale: locale === 'tr' ? 'tr-TR' : 'en-GB' };
}

/** Date-time in the user's zone, e.g. "22 Nov 2026, 14:05". */
export function formatDateTime(iso: string | null | undefined): string {
    if (!iso) return '—';
    return new Intl.DateTimeFormat(prefs.locale, {
        dateStyle: 'medium',
        timeStyle: 'short',
        timeZone: prefs.timeZone,
    }).format(new Date(iso));
}

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
