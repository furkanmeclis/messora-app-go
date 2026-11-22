import 'server-only';

import { cache } from 'react';
import { redirect } from 'next/navigation';
import { api, ApiError } from './api';
import { getSessionToken } from './session';
import type { Me } from '@/lib/types';

const loadMe = cache(async (): Promise<Me> => {
    if (!(await getSessionToken())) redirect('/login');
    try {
        return (await api<Me>('/me')).data;
    } catch (err) {
        if (err instanceof ApiError && err.status === 401) {
            redirect('/logout');
        }
        throw err;
    }
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
