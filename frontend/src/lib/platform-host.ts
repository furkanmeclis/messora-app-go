/**
 * The platform's own addresses. Workspaces live at `<slug>.<base>`, the
 * platform admin panel at `admin.<base>`. The base domain comes from
 * MESSORA_BASE_DOMAIN (or TENANT_BASE_DOMAIN, the API's name for it).
 */

/** The configured base domain, lower case, without dots around it. */
export function configuredBaseDomain(
    env: Record<string, string | undefined> = process.env,
): string {
    return (env.MESSORA_BASE_DOMAIN || env.TENANT_BASE_DOMAIN || '')
        .trim()
        .toLowerCase()
        .replace(/^\.+|\.+$/g, '');
}

/** The internal route segment the admin host is served from. */
export const PLATFORM_SEGMENT = '/platform-admin';

/** Host name without port, lower case. */
function hostName(host: string): string {
    return host.trim().toLowerCase().replace(/:\d+$/, '').replace(/\.$/, '');
}

/** Whether host (with or without port) is the admin panel's address. */
export function isAdminHost(host: string | null | undefined, base: string) {
    return !!host && !!base && hostName(host) === `admin.${base}`;
}

/**
 * The CSP source matching every workspace address (their logos appear on
 * the workspace picker): `https://*.messora.tech`, with the port of a
 * development stack.
 */
export function workspaceImageSource(
    host: string,
    https: boolean,
    base: string,
): string {
    if (!base || !/^[a-z0-9.-]+$/.test(base)) return '';
    const port = /:(\d{1,5})$/.exec(host.trim())?.[1];
    return `${https ? 'https' : 'http'}://*.${base}${port ? ':' + port : ''}`;
}
