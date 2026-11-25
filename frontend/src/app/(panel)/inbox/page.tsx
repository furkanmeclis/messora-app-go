import type { Metadata } from 'next';
import { redirect } from 'next/navigation';
import { RealtimeRefresh } from '@/components/realtime';
import { getT } from '@/i18n/server';
import { cn } from '@/lib/utils';
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
import { ConversationList, InboxFilters, Thread } from './inbox-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('inbox.title') };
}

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
            // Only the templates of the account the conversation's number
            // belongs to can be sent from it.
            perms.includes('templates.read')
                ? api<Template[]>(
                      `/templates?status=APPROVED&account_id=${conversation.data.phone_number.account_id}`,
                  )
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
    const t = await getT();
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
    const unread = sp.unread === '1' || sp.unread === 'true';
    const query = new URLSearchParams();
    if (status !== 'all') query.set('status', status);
    if (assignee) query.set('assignee', assignee);
    if (unread) query.set('unread', 'true');
    if (q) query.set('q', q);
    // Links in the page keep the UI's own parameters.
    const ui = new URLSearchParams();
    if (status !== 'open') ui.set('status', status);
    if (assignee) ui.set('assignee', assignee);
    if (unread) ui.set('unread', '1');
    if (q) ui.set('q', q);
    const backHref = ui.toString() ? `/inbox?${ui}` : '/inbox';

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
            <RealtimeRefresh
                channel="inbox:all"
                types={[
                    'conversation.updated',
                    'message.created',
                    'message.status',
                ]}
            />
            {/* Full-bleed: fills the content area below the top bar, edge to
                edge (the panel's padding is cancelled). */}
            <div className="-m-4 grid h-[calc(100svh-var(--topbar-h))] min-h-[28rem] overflow-hidden bg-card md:-m-6 lg:grid-cols-[22rem_1fr] xl:-mx-8">
                <section
                    aria-label={t('inbox.conversations')}
                    className={cn(
                        'flex min-h-0 min-w-0 flex-col border-r',
                        selected && 'hidden lg:flex',
                    )}
                >
                    <InboxFilters
                        title={t('inbox.title')}
                        unreadTotal={list.data.reduce(
                            (n, c) => n + c.unread_count,
                            0,
                        )}
                        status={status}
                        assignee={assignee}
                        unread={unread}
                        q={q}
                    />
                    <ConversationList
                        key={query.toString()}
                        initial={list.data}
                        initialCursor={list.nextCursor}
                        query={ui.toString()}
                        selected={selected}
                    />
                </section>
                <section
                    aria-label={t('inbox.conversation')}
                    className={cn(
                        'min-h-0 min-w-0',
                        !selected && 'hidden lg:block',
                    )}
                >
                    <Thread
                        key={selected ?? 'none'}
                        selected={selected}
                        data={thread}
                        me={me.user.id}
                        users={users.map((u) => ({ id: u.id, name: u.name }))}
                        canReply={can(me, 'conversations.reply')}
                        canAssign={can(me, 'conversations.assign')}
                        canNote={can(me, 'contacts.update')}
                        backHref={backHref}
                    />
                </section>
            </div>
        </>
    );
}
