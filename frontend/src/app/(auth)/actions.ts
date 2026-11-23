'use server';

import { redirect } from 'next/navigation';
import { api } from '@/lib/server/api';
import { field, runAction, safeNext } from '@/lib/server/actions';
import {
    clearMfaToken,
    getMfaToken,
    setMfaToken,
    setSessionToken,
} from '@/lib/server/session';
import type { ActionResult, Me, Schemas } from '@/lib/types';

type SessionCreated = Schemas['SessionCreated'];
type LoginResult = Schemas['LoginResult'];

async function startSession(s: SessionCreated) {
    await setSessionToken(s.session_token, s.expires_at);
    await clearMfaToken();
}

export async function loginAction(
    _prev: ActionResult | null,
    form: FormData,
): Promise<ActionResult> {
    const next = safeNext(field(form, 'next'));
    const res = await runAction(
        async () =>
            (
                await api<LoginResult>('/auth/login', {
                    method: 'POST',
                    auth: 'none',
                    body: {
                        email: field(form, 'email'),
                        password: String(form.get('password') ?? ''),
                    },
                })
            ).data,
    );
    if (!res.ok) return res;
    if (res.data.status === 'mfa_required' && res.data.mfa_token) {
        await setMfaToken(res.data.mfa_token);
        redirect(`/login/verify?next=${encodeURIComponent(next)}`);
    }
    if (res.data.session) {
        await startSession(res.data.session);
        await land(res.data.session.session_token);
    }
    redirect(next);
}

export async function verifyMfaAction(
    _prev: ActionResult | null,
    form: FormData,
): Promise<ActionResult> {
    const token = await getMfaToken();
    if (!token) redirect('/login');
    const code = field(form, 'code');
    const recovery = field(form, 'recovery_code');
    const res = await runAction(
        async () =>
            (
                await api<SessionCreated>('/auth/login/mfa', {
                    method: 'POST',
                    auth: 'none',
                    body: recovery
                        ? {
                              mfa_token: token,
                              recovery_code: recovery.toLowerCase(),
                          }
                        : { mfa_token: token, code },
                })
            ).data,
    );
    if (!res.ok) {
        if (res.code === 'MFA_EXPIRED') {
            await clearMfaToken();
            redirect('/login?expired=1');
        }
        return res;
    }
    await startSession(res.data);
    redirect(safeNext(field(form, 'next')));
}

export async function setupAction(
    _prev: ActionResult | null,
    form: FormData,
): Promise<ActionResult> {
    const res = await runAction(
        async () =>
            (
                await api<SessionCreated>('/setup', {
                    method: 'POST',
                    auth: 'none',
                    body: {
                        token: field(form, 'token'),
                        name: field(form, 'name'),
                        email: field(form, 'email'),
                        password: String(form.get('password') ?? ''),
                        organization_name:
                            field(form, 'organization_name') || undefined,
                    },
                })
            ).data,
    );
    if (!res.ok) return res;
    await startSession(res.data);
    redirect('/welcome');
}

export async function acceptInvitationAction(
    _prev: ActionResult | null,
    form: FormData,
): Promise<ActionResult> {
    const res = await runAction(
        async () =>
            (
                await api<SessionCreated>('/invitations/accept', {
                    method: 'POST',
                    auth: 'none',
                    body: {
                        token: field(form, 'token'),
                        name: field(form, 'name'),
                        password: String(form.get('password') ?? ''),
                    },
                })
            ).data,
    );
    if (!res.ok) return res;
    await startSession(res.data);
    await land(res.data.session_token);
    redirect('/');
}

export async function forgotPasswordAction(
    _prev: ActionResult | null,
    form: FormData,
): Promise<ActionResult> {
    return runAction(
        () =>
            api('/auth/password/forgot', {
                method: 'POST',
                auth: 'none',
                body: { email: field(form, 'email') },
            }).then(() => undefined),
        'If an account exists for this address, we sent a link to reset the password. Check your inbox.',
    );
}

export async function resetPasswordAction(
    _prev: ActionResult | null,
    form: FormData,
): Promise<ActionResult> {
    const res = await runAction(() =>
        api('/auth/password/reset', {
            method: 'POST',
            auth: 'none',
            body: {
                token: field(form, 'token'),
                password: String(form.get('password') ?? ''),
            },
        }).then(() => undefined),
    );
    if (!res.ok) return res;
    redirect('/login?reset=1');
}

/**
 * Sends a freshly signed-in user straight to where they may go. Users whose
 * role requires two-factor authentication they have not set up go directly
 * to enrolment: redirecting to a panel page that then redirects again
 * left the client router refetching an empty page in a loop.
 */
async function land(token: string): Promise<void> {
    let pending = false;
    try {
        pending = (await api<Me>('/me', { token })).data.mfa_required;
    } catch {
        // Unknown: the panel checks again on the next page.
    }
    if (pending) redirect('/account/security?enroll=1');
}
