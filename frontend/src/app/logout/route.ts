import { NextResponse } from 'next/server';
import { api } from '@/lib/server/api';
import { clearSession, getSessionToken } from '@/lib/server/session';
import { headers } from 'next/headers';
import { requestOrigin } from '@/lib/server/tenant-host';

/**
 * Ends the session (best effort on the API side) and clears the cookies.
 * Also used when the API reports that a session has expired.
 */
async function logout() {
    if (await getSessionToken()) {
        await api('/auth/logout', { method: 'POST' }).catch(() => undefined);
    }
    await clearSession();
    return NextResponse.redirect(
        `${requestOrigin(await headers())}/login`,
        303,
    );
}

export const GET = logout;
export const POST = logout;
