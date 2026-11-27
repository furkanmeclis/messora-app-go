import 'server-only';

import { cache } from 'react';
import { redirect } from 'next/navigation';
import { api, ApiError, type ApiResult } from './api';
import { getPlatformToken } from './session';
import type { Schemas } from '@/lib/types';

export type PlatformMe = Schemas['PlatformMe'];

type Options = Parameters<typeof api>[1];

/** Calls the Go API with the platform admin's session (admin host). */
export function papi<T>(path: string, o: Options = {}): Promise<ApiResult<T>> {
    return api<T>(path, { auth: 'platform', ...o });
}

/** The signed-in platform admin, or null (cached per render). */
export const tryPlatformMe = cache(
    async (): Promise<PlatformMe | null | 'invalid'> => {
        if (!(await getPlatformToken())) return null;
        try {
            return (await papi<PlatformMe>('/platform/me')).data;
        } catch (err) {
            if (err instanceof ApiError && err.status === 401) return 'invalid';
            throw err;
        }
    },
);

/**
 * The signed-in platform admin. Without a session: the sign-in page; with
 * an ended one: sign-out (clears the cookie). Admins without two-factor
 * authentication must enrol first.
 */
export async function requirePlatformMe(
    opts: { allowMfaPending?: boolean } = {},
): Promise<PlatformMe> {
    const me = await tryPlatformMe();
    if (me === null) redirect('/login');
    if (me === 'invalid') redirect('/logout');
    if (me.mfa_required && !opts.allowMfaPending) redirect('/enroll');
    return me;
}
