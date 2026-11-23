'use client';

import { usePathname } from 'next/navigation';
import { Filter, FolderOpen, Tags, Upload, Users } from 'lucide-react';
import { TabNav } from '@/components/tab-nav';
import { useT } from '@/i18n/client';

export type ContactsNavItem = { href: string; label: string };

const ICONS = {
    '/contacts': Users,
    '/contacts/groups': FolderOpen,
    '/contacts/tags': Tags,
    '/contacts/segments': Filter,
    '/contacts/import': Upload,
} as const;

/** Section tabs shared by the contact pages. */
export function ContactsNav({ items }: { items: ContactsNavItem[] }) {
    const t = useT();
    const pathname = usePathname();
    const active = (href: string) =>
        href === '/contacts'
            ? pathname === '/contacts' ||
              /^\/contacts\/[0-9a-f-]{36}$/.test(pathname)
            : pathname.startsWith(href);
    if (items.length < 2) return null;
    return (
        <TabNav
            className="mb-6"
            label={t('contacts.nav.label')}
            items={items.map((i) => ({
                ...i,
                icon: ICONS[i.href as keyof typeof ICONS],
                active: active(i.href),
            }))}
        />
    );
}
