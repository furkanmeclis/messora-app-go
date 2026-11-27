import { RESERVED_SLUGS } from './slug.ts';

/**
 * The workspace a sign-in page belongs to, from the address it was opened
 * on: `acme.messora.tech` → "acme". The bare domain, platform sub-domains
 * (www, app, api…), localhost and IP addresses have no workspace.
 */
export function tenantSlugFromHost(
    host: string | null | undefined,
    baseDomain: string,
): string | null {
    if (!host || !baseDomain) return null;
    const name = host
        .trim()
        .toLowerCase()
        .replace(/:\d+$/, '')
        .replace(/\.$/, '');
    const base = baseDomain
        .trim()
        .toLowerCase()
        .replace(/^\.+|\.$/g, '');
    if (!name.endsWith('.' + base)) return null;
    const label = name.slice(0, -(base.length + 1));
    if (!/^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/.test(label)) return null;
    if (RESERVED_SLUGS.includes(label)) return null;
    return label;
}
