import { NextResponse } from 'next/server';
import { api } from '@/lib/server/api';
import { clearSession, getSessionToken } from '@/lib/server/session';
import { serverConfig } from '@/lib/server/config';

/**
 * Ends the session (best effort on the API side) and clears the cookies.
 * Also used when the API reports that a session has expired.
 */
async function logout() {
    if (await getSessionToken()) {
        await api('/auth/logout', { method: 'POST' }).catch(() => undefined);
    }
    await clearSession();
    return NextResponse.redirect(`${serverConfig().publicAppUrl}/login`, 303);
}

export const GET = logout;
export const POST = logout;
