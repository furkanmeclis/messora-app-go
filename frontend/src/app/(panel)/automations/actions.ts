'use server';

import { revalidatePath } from 'next/cache';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import type {
    ActionResult,
    Automation,
    AutomationRun,
    Schemas,
} from '@/lib/types';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

function check(id: string) {
    if (!UUID.test(id)) throw new Error('invalid id');
}

export type AutomationInput = Schemas['CreateAutomationRequest'];

export async function createAutomationAction(
    input: AutomationInput,
): Promise<ActionResult<Automation>> {
    const t = await getT();
    const res = await runAction(
        async () =>
            (
                await api<Automation>('/automations', {
                    method: 'POST',
                    body: input,
                })
            ).data,
        t('automations.toast.created'),
    );
    revalidatePath('/automations');
    return res;
}

/**
 * Saves changes. The trigger and steps are only sent when they changed,
 * because redefining an automation cancels its runs in progress.
 */
export async function updateAutomationAction(
    id: string,
    patch: Schemas['UpdateAutomationRequest'],
): Promise<ActionResult<Automation>> {
    check(id);
    const t = await getT();
    const res = await runAction(
        async () =>
            (
                await api<Automation>(`/automations/${id}`, {
                    method: 'PATCH',
                    body: patch,
                })
            ).data,
        t('automations.toast.saved'),
    );
    revalidatePath('/automations', 'layout');
    return res;
}

export async function setActiveAction(
    id: string,
    active: boolean,
): Promise<ActionResult> {
    check(id);
    const t = await getT();
    const res = await runAction(
        () =>
            api(`/automations/${id}`, {
                method: 'PATCH',
                body: { active },
            }).then(() => undefined),
        active ? t('automations.toast.on') : t('automations.toast.off'),
    );
    revalidatePath('/automations', 'layout');
    return res;
}

export async function deleteAutomationAction(
    id: string,
): Promise<ActionResult> {
    check(id);
    const t = await getT();
    const res = await runAction(
        () =>
            api(`/automations/${id}`, { method: 'DELETE' }).then(
                () => undefined,
            ),
        t('automations.toast.deleted'),
    );
    revalidatePath('/automations');
    return res;
}

/** Older runs, for "Load more". */
export async function moreRunsAction(
    id: string,
    cursor: string,
): Promise<ActionResult<{ items: AutomationRun[]; next?: string }>> {
    check(id);
    check(cursor);
    return runAction(async () => {
        const r = await api<AutomationRun[]>(
            `/automations/${id}/runs?limit=50&cursor=${cursor}`,
        );
        return { items: r.data, next: r.nextCursor };
    });
}

/** A run with its executed steps. */
export async function getRunAction(
    runId: string,
): Promise<ActionResult<AutomationRun>> {
    check(runId);
    return runAction(
        async () =>
            (await api<AutomationRun>(`/automation-runs/${runId}`)).data,
    );
}
