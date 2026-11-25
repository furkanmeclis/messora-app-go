'use server';

import { revalidatePath } from 'next/cache';
import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import { getT } from '@/i18n/server';
import type { ActionResult, Schemas } from '@/lib/types';

type Profile = Schemas['AIProfile'];

export async function saveProfileAction(
    id: string | null,
    body: Schemas['AIProfileInput'],
): Promise<ActionResult<Profile>> {
    const t = await getT();
    const r = await runAction(
        async () =>
            (
                await api<Profile>(id ? `/ai/profiles/${id}` : '/ai/profiles', {
                    method: id ? 'PUT' : 'POST',
                    body,
                })
            ).data,
        t(id ? 'ai.profiles.saved' : 'ai.profiles.created'),
    );
    revalidatePath('/ai-profiles', 'layout');
    return r;
}

export async function deleteProfileAction(id: string): Promise<ActionResult> {
    const t = await getT();
    const r = await runAction(
        () =>
            api(`/ai/profiles/${id}`, { method: 'DELETE' }).then(
                () => undefined,
            ),
        t('ai.profiles.deleted'),
    );
    revalidatePath('/ai-profiles', 'layout');
    return r;
}

export async function testProfileAction(
    id: string,
    body: Schemas['TestAIProfileRequest'],
): Promise<ActionResult<Schemas['AITestResult']>> {
    return runAction(
        async () =>
            (
                await api<Schemas['AITestResult']>(`/ai/profiles/${id}/test`, {
                    method: 'POST',
                    body,
                })
            ).data,
    );
}
