/** Only a plain host[:port] is forwarded to the API. */
const HOST = /^[a-z0-9.-]+(:\d{1,5})?$/i;

/**
 * The host the browser opened, lowercased, or null when it is missing or
 * malformed. The edge proxy (Caddy) passes Host through unchanged and sets
 * X-Forwarded-Host to the same value (replacing any a client sent); the
 * forwarded header wins because Next.js re-requests itself for server
 * action redirects, and that internal request carries the server's own
 * address in Host but the original request's other headers.
 */
export function requestHost(h: Headers): string | null {
    const forwarded = h.get('x-forwarded-host')?.split(',')[0]?.trim();
    const host = forwarded || h.get('host')?.trim();
    return host && HOST.test(host) ? host.toLowerCase() : null;
}
