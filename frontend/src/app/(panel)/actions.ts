'use server';

import QRCode from 'qrcode';
import { revalidatePath } from 'next/cache';
import { redirect } from 'next/navigation';
import { api } from '@/lib/server/api';
import { field, runAction } from '@/lib/server/actions';
import type { ActionResult, Schemas } from '@/lib/types';
import { isLocale } from '@/i18n/config';
import { getT } from '@/i18n/server';

// ── Re-authentication ──────────────────────────────────────────────────────

export async function reauthAction(
    _prev: ActionResult<string> | null,
    form: FormData,
): Promise<ActionResult<string>> {
    const code = field(form, 'code');
    const password = String(form.get('password') ?? '');
    return runAction(async () => {
        const r = await api<Schemas['ReauthResult']>('/auth/reauth', {
            method: 'POST',
            body: code ? { code } : { password },
        });
        return r.data.valid_until;
    });
}

// ── Account ────────────────────────────────────────────────────────────────

export async function updateProfileAction(
    _prev: ActionResult | null,
    form: FormData,
): Promise<ActionResult> {
    const res = await runAction(
        () =>
            api('/me', {
                method: 'PATCH',
                body: {
                    name: field(form, 'name'),
                    timezone: field(form, 'timezone'),
                    locale: field(form, 'locale'),
                },
            }).then(() => undefined),
        (await getT())('account.profile.saved'),
    );
    revalidatePath('/', 'layout');
    return res;
}

export async function changePasswordAction(
    _prev: ActionResult | null,
    form: FormData,
): Promise<ActionResult> {
    return runAction(
        () =>
            api('/me/password', {
                method: 'POST',
                body: {
                    current_password: String(
                        form.get('current_password') ?? '',
                    ),
                    new_password: String(form.get('new_password') ?? ''),
                },
            }).then(() => undefined),
        (await getT())('account.password.changed'),
    );
}

export async function startTotpAction(): Promise<
    ActionResult<{ secret: string; qr: string }>
> {
    return runAction(async () => {
        const r = await api<Schemas['TotpSetup']>('/me/mfa/totp', {
            method: 'POST',
        });
        const qr = await QRCode.toDataURL(r.data.otpauth_uri, {
            margin: 1,
            width: 220,
        });
        return { secret: r.data.secret, qr };
    });
}

export async function confirmTotpAction(
    _prev: ActionResult<string[]> | null,
    form: FormData,
): Promise<ActionResult<string[]>> {
    const res = await runAction(async () => {
        const r = await api<Schemas['RecoveryCodes']>('/me/mfa/totp/confirm', {
            method: 'POST',
            body: { code: field(form, 'code') },
        });
        return r.data.codes;
    });
    revalidatePath('/', 'layout');
    return res;
}

export async function disableTotpAction(): Promise<ActionResult> {
    const res = await runAction(
        () => api('/me/mfa/totp', { method: 'DELETE' }).then(() => undefined),
        (await getT())('account.twoFactor.turnedOff'),
    );
    revalidatePath('/account/security');
    return res;
}

export async function regenerateCodesAction(): Promise<ActionResult<string[]>> {
    return runAction(async () => {
        const r = await api<Schemas['RecoveryCodes']>(
            '/me/mfa/recovery-codes',
            { method: 'POST' },
        );
        return r.data.codes;
    });
}

export async function revokeSessionAction(id: string): Promise<ActionResult> {
    const res = await runAction(
        () =>
            api(`/me/sessions/${encodeURIComponent(id)}`, {
                method: 'DELETE',
            }).then(() => undefined),
        (await getT())('account.sessions.ended'),
    );
    revalidatePath('/account/security');
    return res;
}

export async function revokeOtherSessionsAction(): Promise<ActionResult> {
    const res = await runAction(
        () => api('/me/sessions', { method: 'DELETE' }).then(() => undefined),
        (await getT())('account.sessions.othersEnded'),
    );
    revalidatePath('/account/security');
    return res;
}

// ── Users and invitations ──────────────────────────────────────────────────

export async function updateUserAction(
    id: string,
    patch: { role?: string; status?: 'active' | 'disabled' },
): Promise<ActionResult> {
    const res = await runAction(
        () =>
            api(`/users/${encodeURIComponent(id)}`, {
                method: 'PATCH',
                body: patch,
            }).then(() => undefined),
        (await getT())('users.updated'),
    );
    revalidatePath('/users');
    return res;
}

export async function inviteAction(
    _prev: ActionResult<Schemas['InvitationCreated']> | null,
    form: FormData,
): Promise<ActionResult<Schemas['InvitationCreated']>> {
    const res = await runAction(async () => {
        const r = await api<Schemas['InvitationCreated']>('/invitations', {
            method: 'POST',
            idempotencyKey: field(form, 'idempotency_key') || undefined,
            body: { email: field(form, 'email'), role: field(form, 'role') },
        });
        return r.data;
    });
    revalidatePath('/users');
    return res;
}

export async function revokeInvitationAction(
    id: string,
): Promise<ActionResult> {
    const res = await runAction(
        () =>
            api(`/invitations/${encodeURIComponent(id)}`, {
                method: 'DELETE',
            }).then(() => undefined),
        (await getT())('users.invitationRevoked'),
    );
    revalidatePath('/users');
    return res;
}

// ── API keys ───────────────────────────────────────────────────────────────

export async function createApiKeyAction(
    _prev: ActionResult<Schemas['ApiKeyCreated']> | null,
    form: FormData,
): Promise<ActionResult<Schemas['ApiKeyCreated']>> {
    const days = Number(field(form, 'expires_in_days'));
    const res = await runAction(async () => {
        const r = await api<Schemas['ApiKeyCreated']>('/api-keys', {
            method: 'POST',
            idempotencyKey: field(form, 'idempotency_key') || undefined,
            body: {
                name: field(form, 'name'),
                scopes: form.getAll('scopes').map(String),
                expires_in_days: days > 0 ? days : undefined,
            },
        });
        return r.data;
    });
    revalidatePath('/api-keys');
    return res;
}

export async function revokeApiKeyAction(id: string): Promise<ActionResult> {
    const res = await runAction(
        () =>
            api(`/api-keys/${encodeURIComponent(id)}`, {
                method: 'DELETE',
            }).then(() => undefined),
        (await getT())('apiKeys.revoked'),
    );
    revalidatePath('/api-keys');
    return res;
}

// ── Settings ───────────────────────────────────────────────────────────────

export async function updateSettingsAction(
    _prev: ActionResult | null,
    form: FormData,
): Promise<ActionResult> {
    const res = await runAction(
        () =>
            api('/settings', {
                method: 'PATCH',
                body: {
                    organization_name: field(form, 'organization_name'),
                    default_timezone: field(form, 'default_timezone'),
                    mfa_required_roles: form
                        .getAll('mfa_required_roles')
                        .map(String),
                    session_idle_hours: Number(
                        field(form, 'session_idle_hours'),
                    ),
                    session_lifetime_days: Number(
                        field(form, 'session_lifetime_days'),
                    ),
                },
            }).then(() => undefined),
        'Settings saved.',
    );
    revalidatePath('/', 'layout');
    return res;
}

// ── Audit log ──────────────────────────────────────────────────────────────

export async function loadAuditPage(
    cursor: string | undefined,
    action: string | undefined,
): Promise<ActionResult<{ entries: Schemas['AuditLog'][]; next?: string }>> {
    return runAction(async () => {
        const q = new URLSearchParams({ limit: '50' });
        if (cursor) q.set('cursor', cursor);
        if (action) q.set('action', action);
        const r = await api<Schemas['AuditLog'][]>(`/audit-logs?${q}`);
        return { entries: r.data, next: r.nextCursor };
    });
}

export async function signOutAction() {
    redirect('/logout');
}

/** Switches the signed-in user's panel language. */
export async function setLanguageAction(locale: string): Promise<ActionResult> {
    if (!isLocale(locale)) {
        return {
            ok: false,
            code: 'VALIDATION_ERROR',
            message: 'Unsupported language',
        };
    }
    const res = await runAction(() =>
        api('/me', { method: 'PATCH', body: { locale } }).then(() => undefined),
    );
    revalidatePath('/', 'layout');
    return res;
}
