import { headers } from 'next/headers';
import { NextResponse, type NextRequest } from 'next/server';
import { api } from '@/lib/server/api';
import { clearMfaToken, setSessionToken } from '@/lib/server/session';
import { requestOrigin } from '@/lib/server/tenant-host';
import type { Schemas } from '@/lib/types';

/**
 * Completes a hand-over from another workspace (or a support session from
 * the platform admin panel): the one-time token from the link is exchanged
 * for a session of this workspace. The token works once, for 60 seconds,
 * and only at the address it was issued for.
 */
export async function GET(req: NextRequest) {
    const origin = requestOrigin(await headers());
    const token = req.nextUrl.searchParams.get('token') ?? '';
    if (!token) return NextResponse.redirect(`${origin}/login`, 303);
    try {
        const s = (
            await api<Schemas['SessionCreated']>('/auth/handoff', {
                method: 'POST',
                auth: 'none',
                body: { token },
            })
        ).data;
        await setSessionToken(s.session_token, s.expires_at, false);
        await clearMfaToken();
    } catch {
        return NextResponse.redirect(`${origin}/login?expired=1`, 303);
    }
    return NextResponse.redirect(`${origin}/`, 303);
}
