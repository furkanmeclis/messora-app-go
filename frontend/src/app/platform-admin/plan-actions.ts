'use server';

import { revalidatePath } from 'next/cache';
import { runAction } from '@/lib/server/actions';
import { papi } from '@/lib/server/platform';
import { getT } from '@/i18n/server';
import type { ActionResult, Schemas } from '@/lib/types';

/*
 * Plans, AI keys, the platform's e-mail sender and plan change requests
 * (Phase 17 WP8) in the platform admin panel.
 */

function refresh() {
    revalidatePath('/platform-admin', 'layout');
}

async function call<T>(
    path: string,
    method: 'POST' | 'PUT' | 'PATCH' | 'DELETE',
    body: unknown,
    okKey?: Parameters<Awaited<ReturnType<typeof getT>>>[0],
): Promise<ActionResult<T>> {
    const t = await getT();
    const r = await runAction(
        async () => (await papi<T>(path, { method, body })).data,
        okKey ? t(okKey) : undefined,
    );
    refresh();
    return r;
}

// ── Plans ──────────────────────────────────────────────────────────────

export async function createPlanAction(
    body: Schemas['PlatformPlanInput'],
): Promise<ActionResult<Schemas['PlatformPlan']>> {
    return call('/platform/plans', 'POST', body, 'plans.admin.plans.created');
}

export async function updatePlanAction(
    key: string,
    body: Schemas['PlatformPlanPatch'],
): Promise<ActionResult<Schemas['PlatformPlan']>> {
    return call(
        `/platform/plans/${encodeURIComponent(key)}`,
        'PATCH',
        body,
        'plans.admin.plans.saved',
    );
}

// ── A company's plan and credits ───────────────────────────────────────

export async function setTenantPlanAction(
    id: string,
    body: Schemas['SetTenantPlanRequest'],
): Promise<ActionResult<Schemas['PlatformTenantPlan']>> {
    return call(
        `/platform/tenants/${id}/plan`,
        'PUT',
        body,
        'plans.admin.tenant.assigned',
    );
}

export async function grantCreditsAction(
    id: string,
    body: Schemas['GrantAICreditsRequest'],
): Promise<ActionResult<Schemas['AICreditLot']>> {
    return call(
        `/platform/tenants/${id}/ai-credits`,
        'POST',
        body,
        'plans.admin.tenant.granted_',
    );
}

// ── AI keys ────────────────────────────────────────────────────────────

export async function createPlatformAIProviderAction(
    body: Schemas['CreateAIProviderRequest'],
): Promise<ActionResult<Schemas['PlatformAIProvider']>> {
    return call(
        '/platform/ai-providers',
        'POST',
        body,
        'plans.admin.aiKeys.saved',
    );
}

export async function updatePlatformAIProviderAction(
    id: string,
    body: Schemas['UpdateAIProviderRequest'],
): Promise<ActionResult<Schemas['PlatformAIProvider']>> {
    return call(
        `/platform/ai-providers/${id}`,
        'PATCH',
        body,
        'plans.admin.aiKeys.saved',
    );
}

export async function deletePlatformAIProviderAction(
    id: string,
): Promise<ActionResult<undefined>> {
    const t = await getT();
    const r = await runAction(async () => {
        await papi(`/platform/ai-providers/${id}`, { method: 'DELETE' });
        return undefined;
    }, t('plans.admin.aiKeys.removed'));
    refresh();
    return r;
}

export async function testPlatformAIProviderAction(
    id: string,
): Promise<ActionResult<Schemas['PlatformAIProvider']>> {
    return call(`/platform/ai-providers/${id}/test`, 'POST', undefined);
}

export async function updatePlatformAISettingsAction(
    body: Schemas['UpdatePlatformAISettingsRequest'],
): Promise<ActionResult<Schemas['PlatformAISettings']>> {
    return call(
        '/platform/ai-settings',
        'PATCH',
        body,
        'plans.admin.aiKeys.settingsSaved',
    );
}

// ── E-mail sender ──────────────────────────────────────────────────────

export async function updatePlatformEmailAction(
    body: Schemas['SmtpSettingsInput'],
): Promise<ActionResult<Schemas['PlatformEmail']>> {
    return call('/platform/email', 'PUT', body, 'plans.admin.email.saved');
}

export async function testPlatformEmailAction(
    body: Schemas['SmtpTestRequest'],
): Promise<ActionResult<undefined>> {
    const t = await getT();
    return runAction(async () => {
        await papi('/platform/email/test', { method: 'POST', body });
        return undefined;
    }, t('plans.admin.email.testSent'));
}

// ── Plan change requests ───────────────────────────────────────────────

export async function resolvePlanRequestAction(
    id: string,
    body: Schemas['ResolvePlanRequest'],
): Promise<ActionResult<Schemas['PlatformPlanRequest']>> {
    return call(
        `/platform/plan-requests/${id}/resolve`,
        'POST',
        body,
        'plans.admin.requests.resolved',
    );
}
