'use server';

import QRCode from 'qrcode';
import { redirect } from 'next/navigation';
import { revalidatePath } from 'next/cache';
import { field, runAction } from '@/lib/server/actions';
import { papi } from '@/lib/server/platform';
import {
    clearPlatformMfaToken,
    getPlatformMfaToken,
    setPlatformMfaToken,
    setPlatformToken,
} from '@/lib/server/session';
import { getT } from '@/i18n/server';
import type { ActionResult, Schemas } from '@/lib/types';

/*
 * Server actions of the platform admin panel. They run on the admin host
 * and call /api/v1/platform/* with the platform session.
 */

type Detail = Schemas['PlatformTenantDetail'];

function refresh() {
    revalidatePath('/platform-admin', 'layout');
}

// ── Sign-in ──────────────────────────────────────────────────────────────

export async function platformLoginAction(
    _prev: ActionResult | null,
    form: FormData,
): Promise<ActionResult> {
    const res = await runAction(
        async () =>
            (
                await papi<Schemas['PlatformLoginResult']>(
                    '/platform/auth/login',
                    {
                        method: 'POST',
                        auth: 'none',
                        body: {
                            email: field(form, 'email'),
                            password: String(form.get('password') ?? ''),
                        },
                    },
                )
            ).data,
    );
    if (!res.ok) return res;
    if (res.data.status === 'mfa_required' && res.data.mfa_token) {
        await setPlatformMfaToken(res.data.mfa_token);
        redirect('/login/verify');
    }
    if (res.data.session)
        await setPlatformToken(res.data.session.session_token);
    redirect('/');
}

export async function platformVerifyAction(
    _prev: ActionResult | null,
    form: FormData,
): Promise<ActionResult> {
    const token = await getPlatformMfaToken();
    if (!token) redirect('/login');
    const code = field(form, 'code');
    const recovery = field(form, 'recovery_code');
    const res = await runAction(
        async () =>
            (
                await papi<Schemas['PlatformSessionCreated']>(
                    '/platform/auth/login/mfa',
                    {
                        method: 'POST',
                        auth: 'none',
                        body: recovery
                            ? {
                                  mfa_token: token,
                                  recovery_code: recovery.toLowerCase(),
                              }
                            : { mfa_token: token, code },
                    },
                )
            ).data,
    );
    if (!res.ok) {
        if (res.code === 'MFA_EXPIRED') {
            await clearPlatformMfaToken();
            redirect('/login');
        }
        return res;
    }
    await setPlatformToken(res.data.session_token);
    await clearPlatformMfaToken();
    redirect('/');
}

export async function bootstrapAction(
    _prev: ActionResult | null,
    form: FormData,
): Promise<ActionResult> {
    const res = await runAction(
        async () =>
            (
                await papi<Schemas['PlatformSessionCreated']>(
                    '/platform/auth/bootstrap',
                    {
                        method: 'POST',
                        auth: 'none',
                        body: {
                            token: field(form, 'token'),
                            name: field(form, 'name'),
                            email: field(form, 'email'),
                            password: String(form.get('password') ?? ''),
                        },
                    },
                )
            ).data,
    );
    if (!res.ok) return res;
    await setPlatformToken(res.data.session_token);
    redirect('/enroll');
}

export async function startEnrollAction(): Promise<
    ActionResult<{ secret: string; qr: string }>
> {
    return runAction(async () => {
        const r = await papi<Schemas['TotpSetup']>('/platform/me/mfa/totp', {
            method: 'POST',
        });
        const qr = await QRCode.toDataURL(r.data.otpauth_uri, {
            margin: 1,
            width: 220,
        });
        return { secret: r.data.secret, qr };
    });
}

export async function confirmEnrollAction(
    _prev: ActionResult<string[]> | null,
    form: FormData,
): Promise<ActionResult<string[]>> {
    return runAction(
        async () =>
            (
                await papi<Schemas['RecoveryCodes']>(
                    '/platform/me/mfa/totp/confirm',
                    { method: 'POST', body: { code: field(form, 'code') } },
                )
            ).data.codes,
    );
}

// ── Companies ────────────────────────────────────────────────────────────

export async function createTenantAction(
    body: Schemas['CreatePlatformTenantRequest'],
): Promise<ActionResult<Schemas['PlatformTenantCreated']>> {
    const t = await getT();
    const r = await runAction(
        async () =>
            (
                await papi<Schemas['PlatformTenantCreated']>(
                    '/platform/tenants',
                    { method: 'POST', body },
                )
            ).data,
        t('platform.tenants.created'),
    );
    refresh();
    return r;
}

export async function updateTenantAction(
    id: string,
    body: Schemas['UpdatePlatformTenantRequest'],
): Promise<ActionResult<Detail>> {
    const t = await getT();
    const r = await runAction(
        async () =>
            (
                await papi<Detail>(`/platform/tenants/${id}`, {
                    method: 'PATCH',
                    body,
                })
            ).data,
        t('platform.tenant.saved'),
    );
    refresh();
    return r;
}

export async function suspendTenantAction(
    id: string,
    reason: string,
): Promise<ActionResult<Detail>> {
    const r = await runAction(
        async () =>
            (
                await papi<Detail>(`/platform/tenants/${id}/suspend`, {
                    method: 'POST',
                    body: { reason },
                })
            ).data,
    );
    refresh();
    return r;
}

export async function unsuspendTenantAction(
    id: string,
): Promise<ActionResult<Detail>> {
    const t = await getT();
    const r = await runAction(
        async () =>
            (
                await papi<Detail>(`/platform/tenants/${id}/unsuspend`, {
                    method: 'POST',
                })
            ).data,
        t('platform.tenant.unsuspended'),
    );
    refresh();
    return r;
}

export async function inviteOwnerAction(
    id: string,
    email: string,
): Promise<ActionResult<Schemas['OwnerInvitation']>> {
    const t = await getT();
    const r = await runAction(
        async () =>
            (
                await papi<Schemas['OwnerInvitation']>(
                    `/platform/tenants/${id}/owner-invitations`,
                    { method: 'POST', body: { email } },
                )
            ).data,
        t('platform.tenant.invited'),
    );
    refresh();
    return r;
}

export async function addDomainAction(
    id: string,
    hostname: string,
): Promise<ActionResult<Schemas['PlatformDomain']>> {
    const r = await runAction(
        async () =>
            (
                await papi<Schemas['PlatformDomain']>(
                    `/platform/tenants/${id}/domains`,
                    { method: 'POST', body: { hostname } },
                )
            ).data,
    );
    refresh();
    return r;
}

export async function domainAction(
    domainId: string,
    op: 'verify' | 'primary' | 'delete',
): Promise<ActionResult> {
    const r = await runAction(() =>
        (op === 'delete'
            ? papi(`/platform/domains/${domainId}`, { method: 'DELETE' })
            : papi(`/platform/domains/${domainId}/${op}`, { method: 'POST' })
        ).then(() => undefined),
    );
    refresh();
    return r;
}

export async function startSupportAction(
    id: string,
    body: Schemas['StartSupportSessionRequest'],
): Promise<ActionResult<Schemas['SupportSessionStarted']>> {
    const r = await runAction(
        async () =>
            (
                await papi<Schemas['SupportSessionStarted']>(
                    `/platform/tenants/${id}/support-sessions`,
                    { method: 'POST', body },
                )
            ).data,
    );
    refresh();
    return r;
}

export async function endSupportAction(id: string): Promise<ActionResult> {
    const t = await getT();
    const r = await runAction(
        () =>
            papi(`/platform/support-sessions/${id}/end`, {
                method: 'POST',
            }).then(() => undefined),
        t('platform.support.ended'),
    );
    refresh();
    return r;
}

// ── Accounts ─────────────────────────────────────────────────────────────

export async function userAction(
    id: string,
    op: 'disable' | 'enable' | 'force-password-reset',
): Promise<ActionResult> {
    const t = await getT();
    const r = await runAction(
        () =>
            papi(`/platform/users/${id}/${op}`, { method: 'POST' }).then(
                () => undefined,
            ),
        t('platform.user.done'),
    );
    refresh();
    return r;
}

// ── Settings ─────────────────────────────────────────────────────────────

export async function updatePlatformSettingsAction(
    body: Schemas['UpdatePlatformSettingsRequest'],
): Promise<ActionResult<Schemas['PlatformSettings']>> {
    const t = await getT();
    const r = await runAction(
        async () =>
            (
                await papi<Schemas['PlatformSettings']>('/platform/settings', {
                    method: 'PATCH',
                    body,
                })
            ).data,
        t('platform.settings.saved'),
    );
    refresh();
    return r;
}

// ── Lists with "load more" ───────────────────────────────────────────────

export async function loadTenantsAction(
    query: string,
): Promise<ActionResult<{ rows: Schemas['PlatformTenant'][]; next?: string }>> {
    return runAction(async () => {
        const r = await papi<Schemas['PlatformTenant'][]>(
            `/platform/tenants?${query}`,
        );
        return { rows: r.data, next: r.nextCursor };
    });
}

export async function loadAuditAction(
    cursor: string,
): Promise<
    ActionResult<{ rows: Schemas['PlatformAuditLog'][]; next?: string }>
> {
    return runAction(async () => {
        const r = await papi<Schemas['PlatformAuditLog'][]>(
            `/platform/audit-logs?limit=50&cursor=${encodeURIComponent(cursor)}`,
        );
        return { rows: r.data, next: r.nextCursor };
    });
}
