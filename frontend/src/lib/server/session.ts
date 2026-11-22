import 'server-only';

import { cookies } from 'next/headers';
import { serverConfig } from './config';

/**
 * The session token lives only in an HttpOnly cookie; browser JavaScript can
 * never read it. On HTTPS the `__Host-` prefix pins it to this origin.
 */
function names() {
    const secure = serverConfig().secureCookies;
    return {
        secure,
        session: secure ? '__Host-messora_session' : 'messora_session',
        mfa: secure ? '__Host-messora_mfa' : 'messora_mfa',
    };
}

const base = (secure: boolean) =>
    ({
        httpOnly: true,
        secure,
        sameSite: 'lax',
        path: '/',
    }) as const;

export async function getSessionToken(): Promise<string | undefined> {
    return (await cookies()).get(names().session)?.value;
}

export async function setSessionToken(token: string, expiresAt: string) {
    const n = names();
    (await cookies()).set(n.session, token, {
        ...base(n.secure),
        expires: new Date(expiresAt),
    });
}

export async function clearSession() {
    const n = names();
    const jar = await cookies();
    jar.delete({ name: n.session, path: '/' });
    jar.delete({ name: n.mfa, path: '/' });
}

/** Pending second-factor challenge between the password and code steps. */
export async function setMfaToken(token: string) {
    const n = names();
    (await cookies()).set(n.mfa, token, { ...base(n.secure), maxAge: 300 });
}

export async function getMfaToken(): Promise<string | undefined> {
    return (await cookies()).get(names().mfa)?.value;
}

export async function clearMfaToken() {
    (await cookies()).delete({ name: names().mfa, path: '/' });
}

/** Cookie name, for the edge proxy's signed-in check. */
export function sessionCookieNames() {
    return ['__Host-messora_session', 'messora_session'];
}
