'use server';

import { cookies } from 'next/headers';
import { redirect } from 'next/navigation';
import { api, ApiError } from '@/lib/server/api';
import { errorMessage, field, runAction, safeNext } from '@/lib/server/actions';
import {
    clearMfaToken,
    clearSession,
    getMfaToken,
    setMfaToken,
    setRememberChoice,
    setSessionToken,
    takeRememberChoice,
} from '@/lib/server/session';
import { serverConfig } from '@/lib/server/config';
import { SIGNUP_EMAIL_COOKIE, signupEmail } from '@/lib/server/auth-context';
import type { ActionResult, Me, Schemas } from '@/lib/types';
import { getLocale, getT, LOCALE_COOKIE } from '@/i18n/server';
import { isLocale } from '@/i18n/config';
import {
    AUTH_ERROR_CODES,
    type ResendVerificationRequest,
    type SignupRequest,
    type SignupResponse,
    type SlugAvailability,
    type SwitchWorkspaceResponse,
    type VerifyEmailResponse,
} from '@/lib/auth/contracts';
import { PASSWORD_MIN } from '@/lib/auth/password-strength';
import { validateSlug } from '@/lib/auth/slug';

type SessionCreated = Schemas['SessionCreated'];
type LoginResult = Schemas['LoginResult'];

async function startSession(s: SessionCreated, persistent = true) {
    await setSessionToken(s.session_token, s.expires_at, persistent);
    await clearMfaToken();
}

export async function loginAction(
    _prev: ActionResult | null,
    form: FormData,
): Promise<ActionResult> {
    const next = safeNext(field(form, 'next'));
    const remember = form.get('remember') === 'on';
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
        await setRememberChoice(remember);
        redirect(`/login/verify?next=${encodeURIComponent(next)}`);
    }
    if (res.data.session) {
        await startSession(res.data.session, remember);
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
    await startSession(res.data, await takeRememberChoice());
    redirect(safeNext(field(form, 'next')));
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
                        name: field(form, 'name') || undefined,
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
    _prev: ActionResult<{ email: string }> | null,
    form: FormData,
): Promise<ActionResult<{ email: string }>> {
    const email = field(form, 'email');
    return runAction(
        () =>
            api('/auth/password/forgot', {
                method: 'POST',
                auth: 'none',
                body: { email },
            }).then(() => ({ email })),
        (await getT())('auth.forgot.sent'),
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

export async function signupAction(
    _prev: ActionResult | null,
    form: FormData,
): Promise<ActionResult> {
    const t = await getT();
    const body: SignupRequest = {
        name: field(form, 'name'),
        email: field(form, 'email'),
        password: String(form.get('password') ?? ''),
        company_name: field(form, 'company_name'),
        workspace_slug: field(form, 'workspace_slug').toLowerCase(),
        locale: await getLocale(),
        accept_terms: true,
    };
    // The form checks all of this first; this covers requests without it.
    const fieldErrors: Record<string, string> = {};
    for (const k of ['name', 'email', 'company_name'] as const) {
        if (!body[k]) fieldErrors[k] = t('auth.validation.required');
    }
    if ([...body.password].length < PASSWORD_MIN) {
        fieldErrors.password = t('auth.validation.passwordMin');
    }
    const slugError = validateSlug(body.workspace_slug);
    if (slugError) fieldErrors.workspace_slug = t(`auth.slug.${slugError}`);
    if (form.get('accept_terms') !== 'on') {
        fieldErrors.accept_terms = t('auth.validation.terms');
    }
    if (Object.keys(fieldErrors).length > 0) {
        return {
            ok: false,
            code: 'VALIDATION_ERROR',
            message: t('auth.validation.summary', {
                count: Object.keys(fieldErrors).length,
            }),
            fieldErrors,
        };
    }

    let created: SignupResponse;
    try {
        created = (
            await api<SignupResponse>('/auth/signup', {
                method: 'POST',
                auth: 'none',
                body,
            })
        ).data;
    } catch (err) {
        const res = await runAction(() => Promise.reject(err));
        if (res.ok) throw err;
        if (res.code === AUTH_ERROR_CODES.slugTaken) {
            res.fieldErrors = {
                ...res.fieldErrors,
                workspace_slug: t('auth.slug.taken'),
            };
        } else if (res.code === AUTH_ERROR_CODES.slugReserved) {
            res.fieldErrors = {
                ...res.fieldErrors,
                workspace_slug: t('auth.slug.reserved'),
            };
        } else if (res.code === AUTH_ERROR_CODES.emailTaken) {
            res.fieldErrors = { ...res.fieldErrors, email: res.message };
        }
        return res;
    }
    (await cookies()).set(SIGNUP_EMAIL_COOKIE, created.email || body.email, {
        httpOnly: true,
        secure: serverConfig().secureCookies,
        sameSite: 'lax',
        path: '/',
        maxAge: 60 * 60 * 24,
    });
    redirect('/verify-email/sent');
}

/**
 * Whether a workspace address is free. `available: null` means the API
 * cannot tell (unreachable); the form then says nothing.
 */
export async function checkSlugAction(
    slug: string,
): Promise<{ available: boolean | null; reason?: 'taken' | 'reserved' }> {
    const s = String(slug ?? '').toLowerCase();
    if (validateSlug(s)) return { available: null };
    try {
        const r = await api<SlugAvailability>(
            `/auth/signup/slug-availability?slug=${encodeURIComponent(s)}`,
            { auth: 'none', timeoutMs: 4000 },
        );
        return { available: r.data.available, reason: r.data.reason };
    } catch {
        return { available: null };
    }
}

export type ResendState = {
    status: 'sent' | 'error';
    message?: string;
    /** Seconds until the button works again. */
    cooldown: number;
    /** When the state was made, so the countdown restarts on each send. */
    at: number;
};

const RESEND_COOLDOWN = 60;

export async function resendVerificationAction(
    _prev: ResendState | null,
    form: FormData,
): Promise<ResendState> {
    const email = field(form, 'email') || (await signupEmail()) || '';
    const at = Date.now();
    if (!email) {
        return {
            status: 'error',
            message: (await getT())('auth.validation.email'),
            cooldown: 0,
            at,
        };
    }
    const body: ResendVerificationRequest = { email };
    try {
        await api('/auth/resend-verification', {
            method: 'POST',
            auth: 'none',
            body,
        });
        return { status: 'sent', cooldown: RESEND_COOLDOWN, at };
    } catch (err) {
        if (!(err instanceof ApiError)) throw err;
        return {
            status: 'error',
            message: await errorMessage(err),
            cooldown:
                err.status === 429 ? (err.retryAfter ?? RESEND_COOLDOWN) : 0,
            at,
        };
    }
}

export type VerifyEmailState =
    | { status: 'verified'; workspace: string | null; loginUrl: string }
    | {
          status: 'expired' | 'invalid' | 'already' | 'error';
          message?: string;
      };

export async function verifyEmailAction(
    _prev: VerifyEmailState | null,
    form: FormData,
): Promise<VerifyEmailState> {
    const token = field(form, 'token');
    if (!token) return { status: 'invalid' };
    try {
        const r = await api<VerifyEmailResponse>('/auth/verify-email', {
            method: 'POST',
            auth: 'none',
            body: { token },
        });
        return {
            status: 'verified',
            workspace: r.data.tenant?.name ?? null,
            loginUrl: safeLoginUrl(r.data.login_url),
        };
    } catch (err) {
        if (!(err instanceof ApiError)) throw err;
        switch (err.code) {
            case AUTH_ERROR_CODES.verificationExpired:
                return { status: 'expired' };
            case AUTH_ERROR_CODES.verificationInvalid:
                return { status: 'invalid' };
            case AUTH_ERROR_CODES.alreadyVerified:
                return { status: 'already' };
        }
        return { status: 'error', message: await errorMessage(err) };
    }
}

/** Only http(s) URLs leave the page; anything else signs in here. */
function safeLoginUrl(raw: string | undefined): string {
    if (!raw) return '/login?verified=1';
    try {
        const u = new URL(raw);
        if (u.protocol === 'https:' || u.protocol === 'http:') return u.href;
    } catch {
        // Relative or malformed: fall through.
    }
    return safeNext(raw) === '/' ? '/login?verified=1' : safeNext(raw);
}

/**
 * Hands the person over to another of their workspaces: sessions belong to
 * one workspace, so the target address signs them in with a one-time
 * token (`/auth/handoff`).
 */
export async function switchWorkspaceAction(form: FormData): Promise<void> {
    const r = await api<SwitchWorkspaceResponse>('/auth/switch-workspace', {
        method: 'POST',
        body: { tenant_id: field(form, 'tenant_id') },
    });
    redirect(safeLoginUrl(r.data.redirect_url));
}

export type ConfirmEmailState =
    { status: 'changed' } | { status: 'invalid'; message?: string };

/** Confirms a new e-mail address from the link sent to it. */
export async function confirmEmailChangeAction(
    _prev: ConfirmEmailState | null,
    form: FormData,
): Promise<ConfirmEmailState> {
    const token = field(form, 'token');
    if (!token) return { status: 'invalid' };
    try {
        await api('/auth/email-change/confirm', {
            method: 'POST',
            auth: 'none',
            body: { token },
        });
    } catch (err) {
        if (!(err instanceof ApiError)) throw err;
        return { status: 'invalid', message: await errorMessage(err) };
    }
    // Every session of the account ended, this browser's too.
    await clearSession();
    return { status: 'changed' };
}

/** The language switch on the sign-in screens. */
export async function setLocaleAction(form: FormData): Promise<void> {
    const locale = form.get('locale');
    if (!isLocale(locale)) return;
    (await cookies()).set(LOCALE_COOKIE, locale, {
        sameSite: 'lax',
        path: '/',
        secure: serverConfig().secureCookies,
        maxAge: 60 * 60 * 24 * 365,
    });
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
