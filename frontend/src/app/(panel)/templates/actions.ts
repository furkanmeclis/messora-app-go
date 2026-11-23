'use server';

import { revalidatePath } from 'next/cache';
import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import type { ActionResult, Schemas, Template } from '@/lib/types';
import { getT } from '@/i18n/server';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

/** Submits a template to Meta for review (it starts as PENDING). */
export async function createTemplateAction(
    input: Schemas['CreateTemplateRequest'],
    idempotencyKey: string,
): Promise<ActionResult<Template>> {
    const t = await getT();
    if (!UUID.test(input.account_id)) {
        const message = t('templates.validation.chooseAccount');
        return {
            ok: false,
            code: 'VALIDATION_ERROR',
            message,
            fieldErrors: { account_id: message },
        };
    }
    const res = await runAction(
        async () =>
            (
                await api<Template>('/templates', {
                    method: 'POST',
                    body: input,
                    idempotencyKey,
                    timeoutMs: 30_000,
                })
            ).data,
        t('templates.create.submitted'),
    );
    revalidatePath('/templates');
    return res;
}

export async function deleteTemplateAction(id: string): Promise<ActionResult> {
    if (!UUID.test(id)) throw new Error('invalid id');
    const t = await getT();
    const res = await runAction(
        () =>
            api(`/templates/${id}`, {
                method: 'DELETE',
                timeoutMs: 30_000,
            }).then(() => undefined),
        t('templates.delete.done'),
    );
    revalidatePath('/templates');
    return res;
}
