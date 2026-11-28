'use server';

import { revalidatePath } from 'next/cache';
import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import { getT } from '@/i18n/server';
import type { ActionResult, Schemas } from '@/lib/types';

type Provider = Schemas['AIProvider'];

function refresh() {
    revalidatePath('/settings/ai', 'layout');
    revalidatePath('/ai-profiles', 'layout');
}

export async function saveAISettingsAction(
    body: Schemas['UpdateAISettingsRequest'],
): Promise<ActionResult<Schemas['AISettings']>> {
    const t = await getT();
    const r = await runAction(
        async () =>
            (
                await api<Schemas['AISettings']>('/ai/settings', {
                    method: 'PUT',
                    body,
                })
            ).data,
        t('ai.settings.saved'),
    );
    refresh();
    return r;
}

export async function createProviderAction(
    body: Schemas['CreateAIProviderRequest'],
): Promise<ActionResult<Provider>> {
    const t = await getT();
    const r = await runAction(
        async () =>
            (await api<Provider>('/ai/providers', { method: 'POST', body }))
                .data,
        t('ai.providers.created'),
    );
    refresh();
    return r;
}

export async function updateProviderAction(
    id: string,
    body: Schemas['UpdateAIProviderRequest'],
    message: 'ai.providers.updated' | 'ai.providers.rotated',
): Promise<ActionResult<Provider>> {
    const t = await getT();
    const r = await runAction(
        async () =>
            (
                await api<Provider>(`/ai/providers/${id}`, {
                    method: 'PATCH',
                    body,
                })
            ).data,
        t(message),
    );
    refresh();
    return r;
}

export async function deleteProviderAction(id: string): Promise<ActionResult> {
    const t = await getT();
    const r = await runAction(
        () =>
            api(`/ai/providers/${id}`, { method: 'DELETE' }).then(
                () => undefined,
            ),
        t('ai.providers.deleted'),
    );
    refresh();
    return r;
}

export async function checkProviderAction(
    id: string,
): Promise<ActionResult<Provider>> {
    const r = await runAction(
        async () =>
            (
                await api<Provider>(`/ai/providers/${id}/check`, {
                    method: 'POST',
                })
            ).data,
    );
    refresh();
    return r;
}

export async function syncModelsAction(
    id: string,
): Promise<ActionResult<Schemas['AIModelSync']>> {
    const r = await runAction(
        async () =>
            (
                await api<Schemas['AIModelSync']>(`/ai/providers/${id}/sync`, {
                    method: 'POST',
                })
            ).data,
    );
    refresh();
    return r;
}
