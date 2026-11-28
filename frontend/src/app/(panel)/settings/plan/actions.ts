'use server';

import { revalidatePath } from 'next/cache';
import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import { getT } from '@/i18n/server';
import type { ActionResult, Schemas } from '@/lib/types';

/** Asks Messora for another plan (WP9 brings self-service). */
export async function requestPlanChangeAction(
    body: Schemas['CreatePlanChangeRequest'],
): Promise<ActionResult<Schemas['PlanChangeRequest']>> {
    const t = await getT();
    const res = await runAction(
        async () =>
            (
                await api<Schemas['PlanChangeRequest']>(
                    '/plan/change-requests',
                    { method: 'POST', body },
                )
            ).data,
        t('plans.change.sent'),
    );
    revalidatePath('/settings/plan');
    return res;
}
