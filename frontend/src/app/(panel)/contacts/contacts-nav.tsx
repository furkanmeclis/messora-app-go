'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { cn } from '@/lib/utils';

export type ContactsNavItem = { href: string; label: string };

/** Section tabs shared by the contact pages. */
export function ContactsNav({ items }: { items: ContactsNavItem[] }) {
    const pathname = usePathname();
    const active = (href: string) =>
        href === '/contacts'
            ? pathname === '/contacts' ||
              /^\/contacts\/[0-9a-f-]{36}$/.test(pathname)
            : pathname.startsWith(href);
    if (items.length < 2) return null;
    return (
        <nav aria-label="Contacts sections" className="mb-6 border-b">
            <ul className="-mb-px flex flex-wrap gap-1">
                {items.map((i) => {
                    const on = active(i.href);
                    return (
                        <li key={i.href}>
                            <Link
                                href={i.href}
                                aria-current={on ? 'page' : undefined}
                                className={cn(
                                    'inline-block rounded-t-md border-b-2 px-3 py-2 text-sm outline-none focus-visible:ring-2 focus-visible:ring-ring',
                                    on
                                        ? 'border-primary font-medium text-foreground'
                                        : 'border-transparent text-muted-foreground hover:text-foreground',
                                )}
                            >
                                {i.label}
                            </Link>
                        </li>
                    );
                })}
            </ul>
        </nav>
    );
}
