'use server';

import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import { getT } from '@/i18n/server';
import type { ActionResult, Schemas } from '@/lib/types';

type Price = Schemas['AIPrice'];

/*
 * The AI price table is platform data: only the platform admin panel
 * changes it (workspaces read it). These actions run on the admin host
 * with the platform session.
 */

export async function createPriceAction(
    body: Schemas['CreateAIPriceRequest'],
): Promise<ActionResult<Price>> {
    const t = await getT();
    return runAction(
        async () =>
            (
                await api<Price>('/platform/ai-prices', {
                    method: 'POST',
                    body,
                    auth: 'platform',
                })
            ).data,
        t('ai.prices.created'),
    );
}

export async function updatePriceAction(
    id: string,
    body: Schemas['UpdateAIPriceRequest'],
): Promise<ActionResult<Price>> {
    const t = await getT();
    return runAction(
        async () =>
            (
                await api<Price>(`/platform/ai-prices/${id}`, {
                    method: 'PATCH',
                    body,
                    auth: 'platform',
                })
            ).data,
        t('ai.prices.saved'),
    );
}

export async function deletePriceAction(id: string): Promise<ActionResult> {
    const t = await getT();
    return runAction(
        () =>
            api(`/platform/ai-prices/${id}`, {
                method: 'DELETE',
                auth: 'platform',
            }).then(() => undefined),
        t('ai.prices.deleted'),
    );
}
