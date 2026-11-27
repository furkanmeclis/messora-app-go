import 'server-only';

import { requestHost } from '@/lib/host';
import { serverConfig } from './config';

export { requestHost };

/**
 * The header that tells the Go API which address the browser opened. The
 * panel reaches the API on its internal address, so the API cannot see the
 * original Host; it believes this header only next to the panel secret and
 * serves the tenant (the company) of that host. Without it the API answers
 * 404, so a request never falls back to some default tenant.
 */
export const TENANT_HOST_HEADER = 'X-Messora-Tenant-Host';

/** Sets the tenant host header on `out` from the incoming headers. */
export function forwardHost(incoming: Headers, out: Headers): void {
    const host = requestHost(incoming);
    if (host) out.set(TENANT_HOST_HEADER, host);
}

/**
 * The origin the browser opened (scheme from the edge proxy or
 * PUBLIC_APP_URL, host from the request), for redirects of route handlers:
 * every workspace and the admin panel have their own address.
 */
export function requestOrigin(incoming: Headers): string {
    const fallback = serverConfig().publicAppUrl;
    const host = requestHost(incoming);
    if (!host) return fallback;
    const forwarded = incoming.get('x-forwarded-proto')?.split(',')[0]?.trim();
    const proto =
        forwarded === 'https' || forwarded === 'http'
            ? forwarded
            : new URL(fallback).protocol.replace(':', '');
    return `${proto}://${host}`;
}
