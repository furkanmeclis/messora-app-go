import 'server-only';

import { cache } from 'react';
import { redirect } from 'next/navigation';
import { api, ApiError } from './api';
import { getSessionToken } from './session';
import type { Me } from '@/lib/types';

type MeResult =
    | { me: Me }
    | { me: null; reason: 'signed-out' }
    | { me: null; reason: 'invalid' }
    | { me: null; reason: 'error'; error: unknown };

/** The signed-in user for this request, without redirecting (cached). */
export const tryMe = cache(async (): Promise<MeResult> => {
    if (!(await getSessionToken())) return { me: null, reason: 'signed-out' };
    try {
        return { me: (await api<Me>('/me')).data };
    } catch (err) {
        if (
            err instanceof ApiError &&
            (err.status === 401 || err.code === 'TENANT_SUSPENDED')
        ) {
            // A suspended workspace signs everyone out; its sign-in page
            // says why.
            return { me: null, reason: 'invalid' };
        }
        return { me: null, reason: 'error', error: err };
    }
});

const loadMe = cache(async (): Promise<Me> => {
    const r = await tryMe();
    if (r.me) return r.me;
    if (r.reason === 'signed-out') redirect('/login');
    if (r.reason === 'invalid') redirect('/logout');
    throw r.error;
});

/**
 * The signed-in user for this request (deduplicated per render). Redirects
 * to sign-in without a valid session, and to two-factor enrolment when the
 * user's role requires it — Next renders layouts and pages in parallel, so
 * every page must check this itself before calling the API.
 */
export async function requireMe(
    opts: { allowMfaPending?: boolean } = {},
): Promise<Me> {
    const me = await loadMe();
    if (me.mfa_required && !opts.allowMfaPending) {
        redirect('/account/security?enroll=1');
    }
    return me;
}

export function can(me: Me, permission: string): boolean {
    return me.permissions.includes(permission);
}

/** Like requireMe, but also requires a permission (404 otherwise). */
export async function requirePermission(permission: string): Promise<Me> {
    const me = await requireMe();
    if (!can(me, permission)) redirect('/');
    return me;
}
