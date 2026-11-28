'use server';

import { revalidatePath } from 'next/cache';
import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import type {
    ActionResult,
    MetaConnection,
    MetaConnectionTest,
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
