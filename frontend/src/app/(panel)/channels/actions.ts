'use server';

import { revalidatePath } from 'next/cache';
import { api } from '@/lib/server/api';
import { field, runAction } from '@/lib/server/actions';
import type { ActionResult, SyncResult, WhatsAppAccount } from '@/lib/types';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

function check(id: string) {
    if (!UUID.test(id)) throw new Error('invalid id');
}

/** Connects an account; secrets go straight to the API and never back. */
export async function connectAccountAction(
    form: FormData,
): Promise<ActionResult<WhatsAppAccount>> {
    const appId = field(form, 'app_id');
    const appSecret = String(form.get('app_secret') ?? '').trim();
    const res = await runAction(
        async () =>
            (
                await api<WhatsAppAccount>('/channels/accounts', {
                    method: 'POST',
                    idempotencyKey: field(form, 'idempotency_key') || undefined,
                    // Meta checks the token, then numbers and templates are
                    // imported: this can take a while.
                    timeoutMs: 60_000,
                    body: {
                        name: field(form, 'name'),
                        waba_id: field(form, 'waba_id'),
                        access_token: String(
                            form.get('access_token') ?? '',
                        ).trim(),
                        app_id: appId || undefined,
                        app_secret: appSecret || undefined,
                    },
                })
            ).data,
        'Account connected. Phone numbers and templates were imported.',
    );
    revalidatePath('/channels');
    return res;
}

export type AccountPatch = {
    name?: string;
    access_token?: string;
    app_secret?: string;
    status?: 'active' | 'disabled';
};

export async function updateAccountAction(
    id: string,
    patch: AccountPatch,
): Promise<ActionResult> {
    check(id);
    const body: AccountPatch = {};
    if (patch.name !== undefined) body.name = patch.name.trim();
    if (patch.access_token) body.access_token = patch.access_token.trim();
    if (patch.app_secret) body.app_secret = patch.app_secret.trim();
    if (patch.status) body.status = patch.status;
    const message = body.status
        ? body.status === 'disabled'
            ? 'Account disabled.'
            : 'Account enabled.'
        : body.access_token || body.app_secret
          ? 'Credentials updated.'
          : 'Account updated.';
    const res = await runAction(
        () =>
            api(`/channels/accounts/${id}`, {
                method: 'PATCH',
                body,
                timeoutMs: 30_000,
            }).then(() => undefined),
        message,
    );
    revalidatePath('/channels');
    return res;
}

export async function syncAccountAction(
    id: string,
): Promise<ActionResult<SyncResult>> {
    check(id);
    const res = await runAction(async () => {
        const r = await api<SyncResult>(`/channels/accounts/${id}/sync`, {
            method: 'POST',
            timeoutMs: 60_000,
        });
        return r.data;
    });
    revalidatePath('/channels');
    revalidatePath('/templates');
    return res;
}

export async function disconnectAccountAction(
    id: string,
): Promise<ActionResult> {
    check(id);
    const res = await runAction(
        () =>
            api(`/channels/accounts/${id}`, { method: 'DELETE' }).then(
                () => undefined,
            ),
        'Account disconnected.',
    );
    revalidatePath('/channels');
    return res;
}

export async function setDefaultNumberAction(
    id: string,
): Promise<ActionResult> {
    check(id);
    const res = await runAction(
        () =>
            api(`/channels/phone-numbers/${id}/default`, {
                method: 'POST',
            }).then(() => undefined),
        'Default number changed.',
    );
    revalidatePath('/channels');
    return res;
}
