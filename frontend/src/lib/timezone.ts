/**
 * Wall-clock times in a named IANA time zone (the user's profile setting),
 * independent of the browser's own zone. Values use the
 * `<input type="datetime-local">` format: YYYY-MM-DDTHH:mm.
 */

function parts(ts: number, timeZone: string) {
    const p = new Intl.DateTimeFormat('en-US', {
        timeZone,
        hourCycle: 'h23',
        year: 'numeric',
        month: '2-digit',
        day: '2-digit',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
    }).formatToParts(new Date(ts));
    const get = (t: Intl.DateTimeFormatPartTypes) =>
        Number(p.find((x) => x.type === t)?.value ?? 0);
    return {
        year: get('year'),
        month: get('month'),
        day: get('day'),
        hour: get('hour') % 24,
        minute: get('minute'),
        second: get('second'),
    };
}

/** Milliseconds the zone is ahead of UTC at instant ts. */
function offset(ts: number, timeZone: string): number {
    const x = parts(ts, timeZone);
    const asUtc = Date.UTC(
        x.year,
        x.month - 1,
        x.day,
        x.hour,
        x.minute,
        x.second,
    );
    return asUtc - Math.floor(ts / 1000) * 1000;
}

/** The instant a wall-clock time in timeZone denotes; null if invalid. */
export function zonedToUtc(local: string, timeZone: string): Date | null {
    const m = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})$/.exec(local);
    if (!m) return null;
    const guess = Date.UTC(
        Number(m[1]),
        Number(m[2]) - 1,
        Number(m[3]),
        Number(m[4]),
        Number(m[5]),
    );
    try {
        const first = offset(guess, timeZone);
        let utc = guess - first;
        const second = offset(utc, timeZone);
        if (second !== first) utc = guess - second;
        return new Date(utc);
    } catch {
        return null; // unknown time zone
    }
}

const pad = (n: number) => String(n).padStart(2, '0');

/** The wall-clock time of an instant in timeZone. */
export function utcToZoned(iso: string | Date, timeZone: string): string {
    const ts = typeof iso === 'string' ? Date.parse(iso) : iso.getTime();
    try {
        const x = parts(ts, timeZone);
        return `${x.year}-${pad(x.month)}-${pad(x.day)}T${pad(x.hour)}:${pad(x.minute)}`;
    } catch {
        return '';
    }
}

/** e.g. "22 Nov 2026, 14:05 (Europe/Istanbul)". */
export function formatInZone(iso: string | Date, timeZone: string): string {
    try {
        return `${new Intl.DateTimeFormat(undefined, {
            dateStyle: 'medium',
            timeStyle: 'short',
            timeZone,
        }).format(new Date(iso))} (${timeZone})`;
    } catch {
        return new Date(iso).toISOString();
    }
}
