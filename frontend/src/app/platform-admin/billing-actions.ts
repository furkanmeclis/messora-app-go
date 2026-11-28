'use server';

import { revalidatePath } from 'next/cache';
import { runAction } from '@/lib/server/actions';
import { papi } from '@/lib/server/platform';
import { getT } from '@/i18n/server';
import type { ActionResult, Schemas } from '@/lib/types';

/*
 * Billing in the platform admin panel (Phase 17 WP9): the payments inbox,
 * proformas, invoices, settings and a company's billing actions.
 */

type Key = Parameters<Awaited<ReturnType<typeof getT>>>[0];

async function call<T>(
    path: string,
    method: 'POST' | 'PUT' | 'PATCH',
    body: unknown,
    okKey?: Key,
): Promise<ActionResult<T>> {
    const t = await getT();
    const r = await runAction(
        async () => (await papi<T>(path, { method, body })).data,
        okKey ? t(okKey) : undefined,
    );
    revalidatePath('/platform-admin', 'layout');
    return r;
}

export async function recordTransferAction(
    body: Schemas['CreateBankTransferRequest'],
): Promise<ActionResult<Schemas['BankTransfer']>> {
    return call(
        '/platform/billing/transfers',
        'POST',
        body,
        'billing.admin.payments.recorded',
    );
}

export async function matchTransferAction(
    id: string,
    body: Schemas['MatchBankTransferRequest'],
): Promise<ActionResult<Schemas['BankTransfer']>> {
    return call(
        `/platform/billing/transfers/${encodeURIComponent(id)}/match`,
        'POST',
        body,
        'billing.admin.payments.matched',
    );
}

export async function ignoreTransferAction(
    id: string,
    reason: string,
): Promise<ActionResult<Schemas['BankTransfer']>> {
    return call(
        `/platform/billing/transfers/${encodeURIComponent(id)}/ignore`,
        'POST',
        { reason },
        'billing.admin.payments.ignored',
    );
}

export async function voidProformaAction(
    id: string,
    reason: string,
): Promise<ActionResult<Schemas['Proforma']>> {
    return call(
        `/platform/billing/proformas/${encodeURIComponent(id)}/void`,
        'POST',
        { reason },
        'billing.admin.proformas.voided',
    );
}

export async function recordInvoiceAction(
    id: string,
    body: Schemas['RecordInvoiceRequest'],
): Promise<ActionResult<Schemas['Invoice']>> {
    return call(
        `/platform/billing/invoices/${encodeURIComponent(id)}`,
        'PATCH',
        body,
        'billing.admin.invoices.recorded',
    );
}

export async function saveBillingSettingsAction(
    body: Schemas['UpdatePlatformBillingSettingsRequest'],
): Promise<ActionResult<Schemas['PlatformBillingSettings']>> {
    return call(
        '/platform/billing/settings',
        'PUT',
        body,
        'billing.admin.settings.saved',
    );
}

export async function billingActionAction(
    tenantId: string,
    body: Schemas['PlatformBillingActionRequest'],
): Promise<ActionResult<Schemas['PlatformTenantBilling']>> {
    return call(
        `/platform/tenants/${encodeURIComponent(tenantId)}/billing/actions`,
        'POST',
        body,
        'billing.admin.tenant.applied',
    );
}
