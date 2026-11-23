import { redirect } from 'next/navigation';
import { getT } from '@/i18n/server';
import { can, requireMe } from '@/lib/server/auth';
import { ContactsNav, type ContactsNavItem } from './contacts-nav';

export default async function ContactsLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    const me = await requireMe();
    if (!can(me, 'contacts.read') && !can(me, 'segments.read')) redirect('/');
    const t = await getT();
    const items: ContactsNavItem[] = [];
    if (can(me, 'contacts.read')) {
        items.push(
            { href: '/contacts', label: t('contacts.nav.contacts') },
            { href: '/contacts/groups', label: t('contacts.nav.groups') },
            { href: '/contacts/tags', label: t('contacts.nav.tags') },
        );
    }
    if (can(me, 'segments.read')) {
        items.push({
            href: '/contacts/segments',
            label: t('contacts.nav.segments'),
        });
    }
    if (can(me, 'contacts.import')) {
        items.push({
            href: '/contacts/import',
            label: t('contacts.nav.imports'),
        });
    }
    return (
        <>
            <ContactsNav items={items} />
            {children}
        </>
    );
}
