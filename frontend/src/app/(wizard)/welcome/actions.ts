'use server';

import { revalidatePath } from 'next/cache';
import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import type {
    ActionResult,
    RoleKey,
    Schemas,
    WhatsAppAccount,
} from '@/lib/types';

/** Connects a WhatsApp Business Account from the wizard. */
export async function connectAccountAction(
    input: Schemas['ConnectWhatsAppAccountRequest'],
): Promise<ActionResult<WhatsAppAccount>> {
    const res = await runAction(
        async () =>
            (
                await api<WhatsAppAccount>('/channels/accounts', {
                    method: 'POST',
                    body: input,
                })
            ).data,
    );
    revalidatePath('/channels');
    return res;
}

export type SentInvitation = {
    email: string;
    ok: boolean;
    message?: string;
    acceptUrl?: string;
    emailSent?: boolean;
};

/** Sends invitations one by one and reports each result. */
export async function inviteTeamAction(
    people: { email: string; role: RoleKey }[],
): Promise<ActionResult<SentInvitation[]>> {
    const out: SentInvitation[] = [];
    for (const p of people) {
        const r = await runAction(
            async () =>
                (
                    await api<Schemas['InvitationCreated']>('/invitations', {
                        method: 'POST',
                        body: p,
                    })
                ).data,
        );
        out.push(
            r.ok
                ? {
                      email: p.email,
                      ok: true,
                      acceptUrl: r.data.accept_url,
                      emailSent: r.data.email_sent,
                  }
                : { email: p.email, ok: false, message: r.message },
        );
    }
    revalidatePath('/users');
    return { ok: true, data: out };
}

/** The owner's own language follows the choice made in the wizard. */
export async function setOwnLocaleAction(locale: 'en' | 'tr'): Promise<void> {
    await runAction(() =>
        api('/me', { method: 'PATCH', body: { locale } }).then(() => undefined),
    );
    revalidatePath('/', 'layout');
}
