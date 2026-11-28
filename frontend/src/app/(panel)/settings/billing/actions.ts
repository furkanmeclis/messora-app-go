'use server';

import { revalidatePath } from 'next/cache';
import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import { getT } from '@/i18n/server';
import type { ActionResult, Schemas } from '@/lib/types';

/*
 * The workspace's billing (Phase 17 WP9): invoice details, plan choice,
 * AI credit packs and message balance top-ups, cancelling a proforma.
 */

function refresh() {
    revalidatePath('/settings/billing');
    revalidatePath('/', 'layout');
}

export async function saveBillingProfileAction(
    body: Schemas['BillingProfileInput'],
): Promise<ActionResult<Schemas['BillingProfile']>> {
    const t = await getT();
    const r = await runAction(
        async () =>
            (
                await api<Schemas['BillingProfile']>('/billing/profile', {
                    method: 'PUT',
                    body,
                })
            ).data,
        t('billing.profile.saved'),
    );
    refresh();
    return r;
}

export async function changeBillingPlanAction(
    body: Schemas['BillingPlanChangeRequest'],
): Promise<ActionResult<Schemas['BillingPlanChangeResult']>> {
    const r = await runAction(
        async () =>
            (
                await api<Schemas['BillingPlanChangeResult']>(
                    '/billing/plan-change',
                    { method: 'POST', body },
                )
            ).data,
    );
    refresh();
    return r;
}

export async function cancelBillingChangeAction(): Promise<
    ActionResult<Schemas['BillingSubscription']>
> {
    const t = await getT();
    const r = await runAction(
        async () =>
            (
                await api<Schemas['BillingSubscription']>(
                    '/billing/plan-change',
                    { method: 'DELETE' },
                )
            ).data,
        t('billing.plan.pendingCanceled'),
    );
    refresh();
    return r;
}

export async function createBillingProformaAction(
    body: Schemas['CreateBillingProformaRequest'],
): Promise<ActionResult<Schemas['Proforma']>> {
    const r = await runAction(
        async () =>
            (
                await api<Schemas['Proforma']>('/billing/proformas', {
                    method: 'POST',
                    body,
                })
            ).data,
    );
    refresh();
    return r;
}

export async function cancelBillingProformaAction(
    id: string,
): Promise<ActionResult<Schemas['Proforma']>> {
    const t = await getT();
    const r = await runAction(
        async () =>
            (
                await api<Schemas['Proforma']>(
                    `/billing/proformas/${encodeURIComponent(id)}/cancel`,
                    { method: 'POST' },
                )
            ).data,
        t('billing.proforma.canceled'),
    );
    refresh();
    return r;
}
