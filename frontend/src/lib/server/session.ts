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
        // The platform admin panel (admin host) has cookies of its own.
        platform: secure ? '__Host-messora_platform' : 'messora_platform',
        platformMfa: secure
            ? '__Host-messora_platform_mfa'
            : 'messora_platform_mfa',
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

/**
 * Stores the session. Unless `persistent` ("keep me signed in"), the
 * cookie has no expiry and the browser drops it when it closes; the API
 * session still ends at `expiresAt` either way.
 */
export async function setSessionToken(
    token: string,
    expiresAt: string,
    persistent = true,
) {
    const n = names();
    (await cookies()).set(n.session, token, {
        ...base(n.secure),
        ...(persistent ? { expires: new Date(expiresAt) } : {}),
    });
}

/** Remembers whether the pending sign-in asked to stay signed in. */
const REMEMBER_COOKIE = 'messora_remember';

export async function setRememberChoice(remember: boolean) {
    const n = names();
    (await cookies()).set(REMEMBER_COOKIE, remember ? '1' : '0', {
        ...base(n.secure),
        maxAge: 300,
    });
}

export async function takeRememberChoice(): Promise<boolean> {
    const jar = await cookies();
    const v = jar.get(REMEMBER_COOKIE)?.value;
    if (v !== undefined) jar.delete({ name: REMEMBER_COOKIE, path: '/' });
    return v !== '0';
}

export async function clearSession() {
    const n = names();
    const jar = await cookies();
    jar.delete({ name: n.session, path: '/' });
    jar.delete({ name: n.mfa, path: '/' });
    jar.delete({ name: n.platform, path: '/' });
    jar.delete({ name: n.platformMfa, path: '/' });
}

// ── Platform admin panel ────────────────────────────────────────────────

export async function getPlatformToken(): Promise<string | undefined> {
    return (await cookies()).get(names().platform)?.value;
}

/** The platform session lives until the browser closes (12 h at most). */
export async function setPlatformToken(token: string) {
    const n = names();
    (await cookies()).set(n.platform, token, base(n.secure));
}

export async function setPlatformMfaToken(token: string) {
    const n = names();
    (await cookies()).set(n.platformMfa, token, {
        ...base(n.secure),
        maxAge: 300,
    });
}

export async function getPlatformMfaToken(): Promise<string | undefined> {
    return (await cookies()).get(names().platformMfa)?.value;
}

export async function clearPlatformMfaToken() {
    (await cookies()).delete({ name: names().platformMfa, path: '/' });
}

/** Platform cookie names, for the edge proxy's signed-in check. */
export function platformCookieNames() {
    return ['__Host-messora_platform', 'messora_platform'];
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
