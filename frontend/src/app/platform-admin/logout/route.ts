import { headers } from 'next/headers';
import { NextResponse } from 'next/server';
import { papi } from '@/lib/server/platform';
import { clearSession, getPlatformToken } from '@/lib/server/session';
import { requestOrigin } from '@/lib/server/tenant-host';

/** Ends the platform session (best effort) and clears its cookies. */
async function logout() {
    if (await getPlatformToken()) {
        await papi('/platform/auth/logout', { method: 'POST' }).catch(
            () => undefined,
        );
    }
    await clearSession();
    return NextResponse.redirect(
        `${requestOrigin(await headers())}/login`,
        303,
    );
}

export const GET = logout;
export const POST = logout;
