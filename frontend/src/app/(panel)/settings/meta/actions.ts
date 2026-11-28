'use server';

import { revalidatePath } from 'next/cache';
import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import type {
    ActionResult,
    MetaConnection,
    MetaConnectionTest,
    TechProviderDisconnect,
    TechProviderOnboarding,
} from '@/lib/types';
import { getT } from '@/i18n/server';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

/** Grace periods offered when the webhook address is replaced (hours). */
const GRACE = new Set([0, 24, 72, 168]);

function refresh() {
    revalidatePath('/settings/meta');
    revalidatePath('/channels');
}

/** Gives the workspace a new webhook address; the old one keeps working for graceHours. */
export async function rotateWebhookKeyAction(
    graceHours: number,
): Promise<ActionResult<MetaConnection>> {
    if (!GRACE.has(graceHours)) throw new Error('invalid grace period');
    const t = await getT();
    const res = await runAction(
        async () =>
            (
                await api<MetaConnection>(
                    '/channels/meta-connection/webhook-key',
                    { method: 'POST', body: { grace_hours: graceHours } },
                )
            ).data,
        t('channels.meta.rotate.done'),
    );
    refresh();
    return res;
}

export async function rotateVerifyTokenAction(): Promise<
    ActionResult<MetaConnection>
> {
    const t = await getT();
    const res = await runAction(
        async () =>
            (
                await api<MetaConnection>(
                    '/channels/meta-connection/verify-token',
                    { method: 'POST' },
                )
            ).data,
        t('channels.meta.verify.done'),
    );
    refresh();
    return res;
}

/** Tests an account end to end with Meta (token, numbers, subscription, webhooks). */
export async function testConnectionAction(
    id: string,
): Promise<ActionResult<MetaConnectionTest>> {
    if (!UUID.test(id)) throw new Error('invalid id');
    const res = await runAction(
        async () =>
            (
                await api<MetaConnectionTest>(`/channels/accounts/${id}/test`, {
                    method: 'POST',
                    timeoutMs: 60_000,
                })
            ).data,
    );
    refresh();
    return res;
}

// ── Messora as Tech Provider (Embedded Signup) ─────────────────────────────

const DIGITS = /^[0-9]{1,32}$/;
const PIN = /^[0-9]{6}$/;

export interface OnboardingRequest {
    code: string;
    waba_id: string;
    phone_number_id: string;
    business_id?: string;
    pin?: string;
}

/**
 * Runs the onboarding after Meta's popup finished. The server checks the
 * ids against the business token; a failed step comes back as a failed
 * onboarding (not an error) so the panel can offer the retry.
 */
export async function startOnboardingAction(
    req: OnboardingRequest,
): Promise<ActionResult<TechProviderOnboarding>> {
    if (
        !req.code ||
        req.code.length > 2048 ||
        !DIGITS.test(req.waba_id) ||
        !DIGITS.test(req.phone_number_id) ||
        (req.business_id && !DIGITS.test(req.business_id)) ||
        (req.pin && !PIN.test(req.pin))
    ) {
        throw new Error('invalid onboarding request');
    }
    const body: Record<string, string> = {
        code: req.code,
        waba_id: req.waba_id,
        phone_number_id: req.phone_number_id,
    };
    if (req.business_id) body.business_id = req.business_id;
    if (req.pin) body.pin = req.pin;
    const res = await runAction(
        async () =>
            (
                await api<TechProviderOnboarding>(
                    '/channels/tech-provider/onboardings',
                    { method: 'POST', body, timeoutMs: 70_000 },
                )
            ).data,
    );
    refresh();
    return res;
}

/** Retries a failed onboarding (a new code after Meta's popup, or a new PIN). */
export async function retryOnboardingAction(
    id: string,
    input: { code?: string; pin?: string },
): Promise<ActionResult<TechProviderOnboarding>> {
    if (!UUID.test(id)) throw new Error('invalid id');
    if (input.pin && !PIN.test(input.pin)) throw new Error('invalid pin');
    if (input.code && input.code.length > 2048) throw new Error('invalid code');
    const body: Record<string, string> = {};
    if (input.code) body.code = input.code;
    if (input.pin) body.pin = input.pin;
    const res = await runAction(
        async () =>
            (
                await api<TechProviderOnboarding>(
                    `/channels/tech-provider/onboardings/${id}/retry`,
                    { method: 'POST', body, timeoutMs: 70_000 },
                )
            ).data,
    );
    refresh();
    return res;
}

/** Abandons an unfinished onboarding. */
export async function cancelOnboardingAction(
    id: string,
): Promise<ActionResult<TechProviderOnboarding>> {
    if (!UUID.test(id)) throw new Error('invalid id');
    const t = await getT();
    const res = await runAction(
        async () =>
            (
                await api<TechProviderOnboarding>(
                    `/channels/tech-provider/onboardings/${id}`,
                    { method: 'DELETE' },
                )
            ).data,
        t('channels.meta.tp.cancelled'),
    );
    refresh();
    return res;
}

/** Disconnects an account from Messora's Meta app (history is kept). */
export async function disconnectTechProviderAction(
    id: string,
): Promise<ActionResult<TechProviderDisconnect>> {
    if (!UUID.test(id)) throw new Error('invalid id');
    const t = await getT();
    const res = await runAction(
        async () =>
            (
                await api<TechProviderDisconnect>(
                    `/channels/accounts/${id}/tech-provider/disconnect`,
                    { method: 'POST', timeoutMs: 60_000 },
                )
            ).data,
        t('channels.meta.tp.disconnected'),
    );
    refresh();
    return res;
}
