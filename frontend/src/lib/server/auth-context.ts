import 'server-only';

import { cache } from 'react';
import { cookies, headers } from 'next/headers';
import { api } from './api';
import { getAppearance } from './appearance';
import type { Schemas } from '@/lib/types';
import { tenantSlugFromHost } from '@/lib/auth/tenant-host';

export type AuthTenant = {
    slug: string;
    /** The workspace's name, or its address when it has none yet. */
    name: string;
    /** Same-origin logo URL, when the workspace uploaded one. */
    logoUrl: string | null;
};

export type AuthContext = {
    /** Set when the page is opened on a workspace address. */
    tenant: AuthTenant | null;
    /** e.g. messora.tech; workspace addresses are <slug>.<baseDomain>. */
    baseDomain: string;
};

/** The address a new account was created with, for "check your e-mail". */
export const SIGNUP_EMAIL_COOKIE = 'messora_signup_email';

/** The address "check your e-mail" talks about, if this browser signed up. */
export async function signupEmail(): Promise<string | null> {
    return (await cookies()).get(SIGNUP_EMAIL_COOKIE)?.value || null;
}

type Capabilities = Schemas['AuthCapabilities'];

/**
 * What the sign-in screens may offer. When the API cannot be reached the
 * page still renders (its form then reports the problem) instead of
 * failing as a whole.
 */
export const getCapabilities = cache(async (): Promise<Capabilities> => {
    try {
        return (await api<Capabilities>('/auth/capabilities', { auth: 'none' }))
            .data;
    } catch {
        return { password_reset: true, setup_required: false };
    }
});

/** The domain workspaces live under (MESSORA_BASE_DOMAIN). */
export function baseDomain(): string {
    return (process.env.MESSORA_BASE_DOMAIN?.trim() || 'messora.tech')
        .toLowerCase()
        .replace(/^\.+/, '');
}

/**
 * Who the sign-in screens are for. On `acme.messora.tech` that is the Acme
 * workspace: the API answers /appearance for the workspace the address
 * belongs to, so its name and logo come from there.
 */
export const getAuthContext = cache(async (): Promise<AuthContext> => {
    const h = await headers();
    const domain = baseDomain();
    const host = h.get('x-forwarded-host')?.split(',')[0] ?? h.get('host');
    const slug = tenantSlugFromHost(host, domain);
    if (!slug) return { tenant: null, baseDomain: domain };
    const a = await getAppearance();
    const named = a.organization_name && a.organization_name !== 'Messora';
    const v = encodeURIComponent(a.logo?.version ?? '');
    return {
        baseDomain: domain,
        tenant: {
            slug,
            name: named ? a.organization_name : slug,
            logoUrl: a.logo ? `/bff/logo?v=${v}` : null,
        },
    };
});
