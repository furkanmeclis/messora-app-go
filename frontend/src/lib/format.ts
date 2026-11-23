/** How dates are shown: the signed-in user's zone and language, and the
 * workspace's date and time formats. */
export type DisplayPrefs = {
    timeZone: string;
    locale: string;
    dateFormat: string;
    timeFormat: '24h' | '12h';
};

/** Fixed fallback (also used before a user is known). */
export const defaultPrefs: DisplayPrefs = {
    timeZone: 'UTC',
    locale: 'en-GB',
    dateFormat: 'dd.MM.yyyy',
    timeFormat: '24h',
};

/** Display preferences from a profile; unknown zones fall back to UTC. */
export function prefsFor(
    timeZone: string,
    locale: string,
    dateFormat = defaultPrefs.dateFormat,
    timeFormat: string = defaultPrefs.timeFormat,
): DisplayPrefs {
    let tz = 'UTC';
    try {
        new Intl.DateTimeFormat('en', { timeZone }).format();
        tz = timeZone;
    } catch {
        // unknown zone: keep UTC
    }
    return {
        timeZone: tz,
        locale: locale === 'tr' ? 'tr-TR' : 'en-GB',
        dateFormat,
        timeFormat: timeFormat === '12h' ? '12h' : '24h',
    };
}

type Parts = Record<'year' | 'month' | 'day' | 'monthName', string>;

function dateParts(d: Date, prefs: DisplayPrefs): Parts {
    const get = (opts: Intl.DateTimeFormatOptions, type: string) =>
        new Intl.DateTimeFormat(prefs.locale, {
            ...opts,
            timeZone: prefs.timeZone,
        })
            .formatToParts(d)
            .find((p) => p.type === type)?.value ?? '';
    return {
        year: get({ year: 'numeric' }, 'year'),
        month: get({ month: '2-digit' }, 'month'),
        day: get({ day: '2-digit' }, 'day'),
        monthName: get({ month: 'short' }, 'month'),
    };
}

/** The date in the workspace format (dd.MM.yyyy, MM/dd/yyyy, d MMM yyyy …). */
export function formatDate(
    iso: string | null | undefined,
    prefs: DisplayPrefs = defaultPrefs,
): string {
    if (!iso) return '—';
    const p = dateParts(new Date(iso), prefs);
    return prefs.dateFormat
        .replace('yyyy', p.year)
        .replace('MMM', p.monthName)
        .replace('MM', p.month)
        .replace('dd', p.day)
        .replace(/\bd\b/, String(Number(p.day)));
}

/** The time in the workspace format (14:05 or 2:05 PM). */
export function formatTime(
    iso: string | null | undefined,
    prefs: DisplayPrefs = defaultPrefs,
    seconds = false,
): string {
    if (!iso) return '—';
    return new Intl.DateTimeFormat(prefs.locale, {
        hour: prefs.timeFormat === '12h' ? 'numeric' : '2-digit',
        minute: '2-digit',
        second: seconds ? '2-digit' : undefined,
        hour12: prefs.timeFormat === '12h',
        timeZone: prefs.timeZone,
    }).format(new Date(iso));
}

/**
 * Date-time, e.g. "22.11.2026 14:05". The zone and locale are explicit so
 * the server and the browser render the same text (no hydration mismatch).
 * Client components use useFormatDateTime() from display-preferences.
 */
export function formatDateTime(
    iso: string | null | undefined,
    prefs: DisplayPrefs = defaultPrefs,
): string {
    if (!iso) return '—';
    return `${formatDate(iso, prefs)} ${formatTime(iso, prefs)}`;
}

/** Date and time with seconds (step timelines are often seconds apart). */
export function formatPrecise(
    iso: string | null | undefined,
    prefs: DisplayPrefs = defaultPrefs,
): string {
    if (!iso) return '—';
    return `${formatDate(iso, prefs)} ${formatTime(iso, prefs, true)}`;
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
        ? formatTime(iso, prefs)
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
