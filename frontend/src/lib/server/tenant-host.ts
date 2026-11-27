import 'server-only';

import { requestHost } from '@/lib/host';

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
