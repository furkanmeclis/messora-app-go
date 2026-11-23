import type { Metadata } from 'next';
import Link from 'next/link';
import { MessageSquare, UserX } from 'lucide-react';
import { Button } from '@/components/ui/button';
import {
    Empty,
    EmptyContent,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import { api, ApiError } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type {
    Contact,
    ContactGroup,
    Conversation,
    Note,
    Tag,
} from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { contactName, UUID_RE as UUID } from '../shared';
import {
    ConsentCard,
    ContactForm,
    ContactFacts,
    DeleteContactButton,
    NotesCard,
} from './contact-detail';

export const metadata: Metadata = { title: 'Contact' };

async function loadContact(id: string): Promise<Contact | null> {
    if (!UUID.test(id)) return null;
    try {
        return (await api<Contact>(`/contacts/${id}`)).data;
    } catch (err) {
        if (err instanceof ApiError && err.status === 404) return null;
        throw err;
    }
}

/** The contact's inbox conversation, when the user may see one. */
async function findConversation(
    contact: Contact,
): Promise<Conversation | undefined> {
    const q = new URLSearchParams({ q: contact.phone, limit: '20' });
    return api<Conversation[]>(`/conversations?${q}`)
        .then((r) => r.data.find((c) => c.contact.id === contact.id))
        .catch(() => undefined);
}

export default async function ContactPage({
    params,
}: {
    params: Promise<{ contactId: string }>;
}) {
    const me = await requirePermission('contacts.read');
    const { contactId } = await params;
    const contact = await loadContact(contactId);
    if (!contact) {
        return (
            <Empty className="py-16">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <UserX />
                    </EmptyMedia>
                    <EmptyTitle>Contact not found</EmptyTitle>
                    <EmptyDescription>
                        It may have been deleted, or the link is wrong.
                    </EmptyDescription>
                </EmptyHeader>
                <EmptyContent>
                    <Button variant="outline" asChild>
                        <Link href="/contacts">Back to contacts</Link>
                    </Button>
                </EmptyContent>
            </Empty>
        );
    }
    const canInbox =
        can(me, 'conversations.read') || can(me, 'conversations.read_all');
    const [notes, tags, groups, conversation] = await Promise.all([
        api<Note[]>(`/contacts/${contact.id}/notes`).then((r) => r.data),
        api<Tag[]>('/tags').then((r) => r.data),
        api<ContactGroup[]>('/groups').then((r) => r.data),
        canInbox ? findConversation(contact) : Promise.resolve(undefined),
    ]);
    const canUpdate = can(me, 'contacts.update');

    return (
        <>
            <PageHeading
                title={contactName(contact)}
                description={contact.phone}
                actions={
                    <div className="flex flex-wrap gap-2">
                        {conversation && (
                            <Button variant="outline" asChild>
                                <Link
                                    href={`/inbox?status=all&c=${conversation.id}`}
                                >
                                    <MessageSquare /> Open conversation
                                </Link>
                            </Button>
                        )}
                        {can(me, 'contacts.delete') && (
                            <DeleteContactButton
                                id={contact.id}
                                name={contactName(contact)}
                            />
                        )}
                    </div>
                }
            />
            <div className="grid gap-6 lg:grid-cols-[1fr_20rem]">
                <ContactForm
                    key={contact.updated_at}
                    contact={contact}
                    tags={tags.map((t) => ({
                        id: t.id,
                        name: t.name,
                        color: t.color,
                    }))}
                    groups={groups.map((g) => ({ id: g.id, name: g.name }))}
                    canUpdate={canUpdate}
                />
                <div className="space-y-6">
                    <ContactFacts contact={contact} />
                    <ConsentCard contact={contact} canUpdate={canUpdate} />
                    <NotesCard
                        contactId={contact.id}
                        notes={notes}
                        canUpdate={canUpdate}
                    />
                </div>
            </div>
        </>
    );
}
