/** Only a plain host[:port] is forwarded to the API. */
const HOST = /^[a-z0-9.-]+(:\d{1,5})?$/i;

/**
 * The host of a request (Caddy passes the browser's Host through
 * unchanged), lowercased, or null when it is missing or malformed.
 */
export function requestHost(h: Headers): string | null {
    const host = h.get('host')?.trim();
    return host && HOST.test(host) ? host.toLowerCase() : null;
}
