'use server';

import { revalidatePath } from 'next/cache';
import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import { getT } from '@/i18n/server';
import type {
    ActionResult,
    AIDraft,
    CampaignParams,
    Conversation,
    ConversationActivity,
    ConversationAIMode,
    ConversationNote,
    InboxMessage,
    Note,
} from '@/lib/types';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

function check(id: string) {
    if (!UUID.test(id)) throw new Error('invalid id');
}

export type Reply =
    | { type: 'text'; body: string }
    | {
          type: 'template';
          name: string;
          language: string;
          params?: CampaignParams;
      }
    | {
          type: 'image' | 'video' | 'audio' | 'document';
          id: string;
          filename?: string;
          caption?: string;
      };

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
            : reply.type === 'template'
              ? {
                    type: 'template',
                    template: {
                        name: reply.name,
                        language: reply.language,
                        ...(reply.params ? { params: reply.params } : {}),
                    },
                }
              : {
                    type: reply.type,
                    media: {
                        id: reply.id,
                        ...(reply.type === 'document' && reply.filename
                            ? { filename: reply.filename }
                            : {}),
                        ...(reply.caption && reply.type !== 'audio'
                            ? { caption: reply.caption }
                            : {}),
                    },
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
        (await getT())(
            assigneeId ? 'inbox.toast.assigned' : 'inbox.toast.unassigned',
        ),
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
        (await getT())(
            status === 'close' ? 'inbox.toast.closed' : 'inbox.toast.reopened',
        ),
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
    // query is the page's own parameters (status absent = open).
    const given = new URLSearchParams(query);
    const q = new URLSearchParams();
    const status = given.get('status') ?? 'open';
    if (status === 'open' || status === 'closed') q.set('status', status);
    for (const k of ['assignee', 'q', 'ai']) {
        const v = given.get(k);
        if (v) q.set(k, v);
    }
    if (given.get('unread')) q.set('unread', 'true');
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
        (await getT())('inbox.toast.noteAdded'),
    );
    revalidatePath('/inbox');
    return res;
}

// ── AI handoff (Phase 16C P6) ──────────────────────────────────────────────

export async function setAIModeAction(
    conversationId: string,
    mode: ConversationAIMode,
): Promise<ActionResult> {
    check(conversationId);
    const res = await runAction(
        () =>
            api<Conversation>(`/conversations/${conversationId}/ai-mode`, {
                method: 'PUT',
                body: { mode },
            }).then(() => undefined),
        (await getT())('inbox.ai.toast.modeSet'),
    );
    revalidatePath('/inbox');
    return res;
}

export async function handOffAction(
    conversationId: string,
    to: 'ai' | 'human',
    assigneeId: string | null = null,
): Promise<ActionResult> {
    check(conversationId);
    if (assigneeId) check(assigneeId);
    const res = await runAction(
        () =>
            api<Conversation>(`/conversations/${conversationId}/handoff`, {
                method: 'POST',
                body:
                    to === 'ai'
                        ? { to, mode: 'auto_reply' }
                        : { to, assignee_id: assigneeId },
            }).then(() => undefined),
        (await getT())(
            to === 'ai' ? 'inbox.ai.toast.toAI' : 'inbox.ai.toast.toHuman',
        ),
    );
    revalidatePath('/inbox');
    return res;
}

export async function takeOverAction(
    conversationId: string,
): Promise<ActionResult> {
    check(conversationId);
    const res = await runAction(
        () =>
            api<Conversation>(`/conversations/${conversationId}/takeover`, {
                method: 'POST',
            }).then(() => undefined),
        (await getT())('inbox.ai.toast.takenOver'),
    );
    revalidatePath('/inbox');
    return res;
}

export async function addConversationNoteAction(
    conversationId: string,
    body: string,
    forAI: boolean,
): Promise<ActionResult> {
    check(conversationId);
    const res = await runAction(
        () =>
            api<ConversationNote>(`/conversations/${conversationId}/notes`, {
                method: 'POST',
                body: { body, for_ai: forAI },
            }).then(() => undefined),
        (await getT())('inbox.toast.noteAdded'),
    );
    revalidatePath('/inbox');
    return res;
}

export async function decideDraftAction(
    conversationId: string,
    draftId: string,
    decision: 'accept' | 'reject',
    body?: string,
): Promise<ActionResult> {
    check(conversationId);
    check(draftId);
    const res = await runAction(
        () =>
            api<AIDraft>(
                `/conversations/${conversationId}/ai-drafts/${draftId}/${decision}`,
                {
                    method: 'POST',
                    ...(decision === 'accept' && body
                        ? { body: { body } }
                        : {}),
                },
            ).then(() => undefined),
        (await getT())(
            decision === 'accept'
                ? 'inbox.ai.toast.draftSent'
                : 'inbox.ai.toast.draftRejected',
        ),
    );
    revalidatePath('/inbox');
    return res;
}

export async function setContactAIAction(
    contactId: string,
    disabled: boolean,
): Promise<ActionResult> {
    check(contactId);
    const res = await runAction(
        () =>
            api(`/contacts/${contactId}/ai`, {
                method: 'PUT',
                body: { disabled },
            }).then(() => undefined),
        (await getT())(
            disabled ? 'inbox.ai.toast.contactOff' : 'inbox.ai.toast.contactOn',
        ),
    );
    revalidatePath('/inbox');
    return res;
}

/** Older activity of a conversation, for "Older". */
export async function olderActivityAction(
    conversationId: string,
    cursor: string,
): Promise<ActionResult<{ items: ConversationActivity[]; next?: string }>> {
    check(conversationId);
    check(cursor);
    return runAction(async () => {
        const r = await api<ConversationActivity[]>(
            `/conversations/${conversationId}/activity?limit=50&cursor=${cursor}`,
        );
        return { items: r.data, next: r.nextCursor };
    });
}
