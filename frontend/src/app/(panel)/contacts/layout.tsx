import { redirect } from 'next/navigation';
import { can, requireMe } from '@/lib/server/auth';
import { ContactsNav, type ContactsNavItem } from './contacts-nav';

export default async function ContactsLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    const me = await requireMe();
    if (!can(me, 'contacts.read') && !can(me, 'segments.read')) redirect('/');
    const items: ContactsNavItem[] = [];
    if (can(me, 'contacts.read')) {
        items.push(
            { href: '/contacts', label: 'Contacts' },
            { href: '/contacts/groups', label: 'Groups' },
            { href: '/contacts/tags', label: 'Tags' },
        );
    }
    if (can(me, 'segments.read')) {
        items.push({ href: '/contacts/segments', label: 'Segments' });
    }
    if (can(me, 'contacts.import')) {
        items.push({ href: '/contacts/import', label: 'Imports' });
    }
    return (
        <>
            <ContactsNav items={items} />
            {children}
        </>
    );
}
