'use server';

import { revalidatePath } from 'next/cache';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { field, runAction } from '@/lib/server/actions';
import type { ActionResult, Schemas } from '@/lib/types';

const PAGE = '/settings/webhooks';
const path = (id: string) => `/webhook-endpoints/${encodeURIComponent(id)}`;

type Endpoint = Schemas['WebhookEndpoint'];
type EndpointSecret = Schemas['WebhookEndpointSecret'];
type Delivery = Schemas['WebhookDelivery'];

function endpointBody(form: FormData) {
    return {
        url: field(form, 'url'),
        description: field(form, 'description'),
        event_types: form
            .getAll('event_types')
            .map(String) as Schemas['WebhookEventType'][],
        active: form.get('active') === 'on',
    };
}

export async function createWebhookAction(
    _prev: ActionResult<EndpointSecret> | null,
    form: FormData,
): Promise<ActionResult<EndpointSecret>> {
    const res = await runAction(async () => {
        const r = await api<EndpointSecret>('/webhook-endpoints', {
            method: 'POST',
            idempotencyKey: field(form, 'idempotency_key') || undefined,
            body: endpointBody(form),
        });
        return r.data;
    });
    revalidatePath(PAGE);
    return res;
}

export async function updateWebhookAction(
    id: string,
    _prev: ActionResult | null,
    form: FormData,
): Promise<ActionResult> {
    const res = await runAction(
        () =>
            api<Endpoint>(path(id), {
                method: 'PATCH',
                body: endpointBody(form),
            }).then(() => undefined),
        (await getT())('webhooks.saved'),
    );
    revalidatePath(PAGE);
    return res;
}

export async function setWebhookActiveAction(
    id: string,
    active: boolean,
): Promise<ActionResult> {
    const res = await runAction(
        () =>
            api<Endpoint>(path(id), {
                method: 'PATCH',
                body: { active },
            }).then(() => undefined),
        (await getT())(active ? 'webhooks.enabled' : 'webhooks.disabledToast'),
    );
    revalidatePath(PAGE);
    return res;
}

export async function rotateWebhookSecretAction(
    id: string,
): Promise<ActionResult<string>> {
    const res = await runAction(async () => {
        const r = await api<EndpointSecret>(`${path(id)}/rotate-secret`, {
            method: 'POST',
        });
        return r.data.secret;
    });
    revalidatePath(PAGE);
    return res;
}

export async function deleteWebhookAction(id: string): Promise<ActionResult> {
    const res = await runAction(
        () => api(path(id), { method: 'DELETE' }).then(() => undefined),
        (await getT())('webhooks.deleted'),
    );
    revalidatePath(PAGE);
    return res;
}

export async function sendWebhookTestAction(id: string): Promise<ActionResult> {
    return runAction(
        () =>
            api<Delivery>(`${path(id)}/test`, { method: 'POST' }).then(
                () => undefined,
            ),
        (await getT())('webhooks.testQueued'),
    );
}

export async function loadWebhookDeliveries(
    id: string,
    cursor: string | undefined,
    status: Schemas['WebhookDeliveryStatus'] | undefined,
): Promise<ActionResult<{ deliveries: Delivery[]; next?: string }>> {
    return runAction(async () => {
        const q = new URLSearchParams({ limit: '25' });
        if (cursor) q.set('cursor', cursor);
        if (status) q.set('status', status);
        const r = await api<Delivery[]>(`${path(id)}/deliveries?${q}`);
        return { deliveries: r.data, next: r.nextCursor };
    });
}

export async function redeliverWebhookAction(
    id: string,
): Promise<ActionResult<Delivery>> {
    const t = await getT();
    return runAction(async () => {
        const r = await api<Delivery>(
            `/webhook-deliveries/${encodeURIComponent(id)}/redeliver`,
            { method: 'POST' },
        );
        return r.data;
    }, t('webhooks.redeliveryQueued'));
}
