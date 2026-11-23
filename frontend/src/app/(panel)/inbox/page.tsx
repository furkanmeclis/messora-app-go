import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { RealtimeRefresh } from '@/components/realtime';
import { api, ApiError } from '@/lib/server/api';
import { can, requireMe } from '@/lib/server/auth';
import type {
    Contact,
    Conversation,
    ConversationEvent,
    InboxMessage,
    Note,
    Template,
    User,
} from '@/lib/types';
import { PageHeading } from '../panel-header';
import { ConversationList, InboxFilters, Thread } from './inbox-client';

export const metadata: Metadata = { title: 'Inbox' };

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

export type ThreadData = {
    conversation: Conversation;
    messages: InboxMessage[];
    nextCursor?: string;
    events: ConversationEvent[];
    templates: Template[];
    contact?: Contact;
    notes: Note[];
};

/** Loads the selected conversation; null when it is not visible. */
async function loadThread(
    id: string,
    perms: string[],
): Promise<ThreadData | null> {
    try {
        const [conversation, messages, events] = await Promise.all([
            api<Conversation>(`/conversations/${id}`),
            api<InboxMessage[]>(`/conversations/${id}/messages?limit=50`),
            api<ConversationEvent[]>(`/conversations/${id}/events`),
        ]);
        const contactId = conversation.data.contact.id;
        const [templates, contact, notes] = await Promise.all([
            perms.includes('templates.read')
                ? api<Template[]>('/templates?status=APPROVED')
                      .then((r) => r.data)
                      .catch(() => [])
                : Promise.resolve([] as Template[]),
            perms.includes('contacts.read')
                ? api<Contact>(`/contacts/${contactId}`)
                      .then((r) => r.data)
                      .catch(() => undefined)
                : Promise.resolve(undefined),
            perms.includes('contacts.read')
                ? api<Note[]>(`/contacts/${contactId}/notes`)
                      .then((r) => r.data)
                      .catch(() => [])
                : Promise.resolve([] as Note[]),
        ]);
        return {
            conversation: conversation.data,
            messages: messages.data,
            nextCursor: messages.nextCursor,
            events: events.data,
            templates,
            contact,
            notes,
        };
    } catch (err) {
        if (err instanceof ApiError && err.status === 404) return null;
        throw err;
    }
}

export default async function InboxPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const me = await requireMe();
    if (!can(me, 'conversations.read') && !can(me, 'conversations.read_all')) {
        redirect('/');
    }
    const sp = await searchParams;
    const status =
        sp.status === 'all'
            ? 'all'
            : sp.status === 'closed'
              ? 'closed'
              : 'open';
    const assignee =
        sp.assignee === 'me' || sp.assignee === 'unassigned' ? sp.assignee : '';
    const q = (sp.q ?? '').trim().slice(0, 100);
    const query = new URLSearchParams();
    if (status !== 'all') query.set('status', status);
    if (assignee) query.set('assignee', assignee);
    if (q) query.set('q', q);

    const selected = sp.c && UUID.test(sp.c) ? sp.c : undefined;
    const [list, thread, users] = await Promise.all([
        api<Conversation[]>(`/conversations?${query}&limit=50`),
        selected ? loadThread(selected, me.permissions) : Promise.resolve(null),
        can(me, 'users.manage') && can(me, 'conversations.assign')
            ? api<User[]>('/users')
                  .then((r) => r.data.filter((u) => u.status === 'active'))
                  .catch(() => [] as User[])
            : Promise.resolve([] as User[]),
    ]);

    return (
        <>
            <PageHeading
                title="Inbox"
                description={
                    can(me, 'conversations.read_all')
                        ? 'Every conversation with your customers.'
                        : 'Conversations assigned to you or to nobody.'
                }
            />
            <RealtimeRefresh
                channel="inbox:all"
                types={[
                    'conversation.updated',
                    'message.created',
                    'message.status',
                ]}
            />
            <div className="grid min-h-[70vh] gap-4 lg:grid-cols-[22rem_1fr]">
                <section
                    aria-label="Conversations"
                    className="flex min-w-0 flex-col gap-3"
                >
                    <InboxFilters status={status} assignee={assignee} q={q} />
                    <ConversationList
                        key={query.toString()}
                        initial={list.data}
                        initialCursor={list.nextCursor}
                        query={query.toString()}
                        selected={selected}
                    />
                </section>
                <section aria-label="Conversation" className="min-w-0">
                    <Thread
                        key={selected ?? 'none'}
                        selected={selected}
                        data={thread}
                        me={me.user.id}
                        users={users.map((u) => ({ id: u.id, name: u.name }))}
                        canReply={can(me, 'conversations.reply')}
                        canAssign={can(me, 'conversations.assign')}
                        canNote={can(me, 'contacts.update')}
                    />
                </section>
            </div>
        </>
    );
}
