'use server';

import { revalidatePath } from 'next/cache';
import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import type {
    ActionResult,
    Conversation,
    InboxMessage,
    Note,
} from '@/lib/types';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

function check(id: string) {
    if (!UUID.test(id)) throw new Error('invalid id');
}

export type Reply =
    | { type: 'text'; body: string }
    | { type: 'template'; name: string; language: string };

/** Sends a message in a conversation (the API applies the 24h rules). */
export async function replyAction(
    conversationId: string,
    reply: Reply,
    idempotencyKey: string,
): Promise<ActionResult> {
    check(conversationId);
    const body =
        reply.type === 'text'
            ? { type: 'text', text: { body: reply.body } }
            : {
                  type: 'template',
                  template: { name: reply.name, language: reply.language },
              };
    const res = await runAction(() =>
        api(`/conversations/${conversationId}/messages`, {
            method: 'POST',
            body,
            idempotencyKey,
        }).then(() => undefined),
    );
    revalidatePath('/inbox');
    return res;
}

export async function assignAction(
    conversationId: string,
    assigneeId: string | null,
): Promise<ActionResult> {
    check(conversationId);
    if (assigneeId) check(assigneeId);
    const res = await runAction(
        () =>
            api<Conversation>(`/conversations/${conversationId}/assignment`, {
                method: 'PUT',
                body: { assignee_id: assigneeId },
            }).then(() => undefined),
        assigneeId ? 'Conversation assigned.' : 'Conversation unassigned.',
    );
    revalidatePath('/inbox');
    return res;
}

export async function setStatusAction(
    conversationId: string,
    status: 'close' | 'reopen',
): Promise<ActionResult> {
    check(conversationId);
    const res = await runAction(
        () =>
            api<Conversation>(`/conversations/${conversationId}/${status}`, {
                method: 'POST',
            }).then(() => undefined),
        status === 'close' ? 'Conversation closed.' : 'Conversation reopened.',
    );
    revalidatePath('/inbox');
    return res;
}

export async function markReadAction(
    conversationId: string,
): Promise<ActionResult> {
    check(conversationId);
    return runAction(() =>
        api<Conversation>(`/conversations/${conversationId}/read`, {
            method: 'POST',
        }).then(() => undefined),
    );
}

/** Older messages of a thread, for "Load older". */
export async function olderMessagesAction(
    conversationId: string,
    cursor: string,
): Promise<ActionResult<{ items: InboxMessage[]; next?: string }>> {
    check(conversationId);
    check(cursor);
    return runAction(async () => {
        const r = await api<InboxMessage[]>(
            `/conversations/${conversationId}/messages?limit=50&cursor=${cursor}`,
        );
        return { items: r.data, next: r.nextCursor };
    });
}

/** More conversations of the current list, for "Load more". */
export async function moreConversationsAction(
    query: string,
    cursor: string,
): Promise<ActionResult<{ items: Conversation[]; next?: string }>> {
    check(cursor);
    const given = new URLSearchParams(query);
    const q = new URLSearchParams();
    for (const k of ['status', 'assignee', 'q', 'unread']) {
        const v = given.get(k);
        if (v) q.set(k, v);
    }
    q.set('cursor', cursor);
    q.set('limit', '50');
    return runAction(async () => {
        const r = await api<Conversation[]>(`/conversations?${q}`);
        return { items: r.data, next: r.nextCursor };
    });
}

export async function addNoteAction(
    contactId: string,
    body: string,
): Promise<ActionResult<Note>> {
    check(contactId);
    const res = await runAction(
        async () =>
            (
                await api<Note>(`/contacts/${contactId}/notes`, {
                    method: 'POST',
                    body: { body },
                })
            ).data,
        'Note added.',
    );
    revalidatePath('/inbox');
    return res;
}
