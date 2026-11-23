'use client';

import { usePathname } from 'next/navigation';
import { activeNav } from '@/lib/navigation';

/** The icon of the current section, in a tinted tile. */
export function SectionIcon() {
    const item = activeNav(usePathname());
    if (!item) return null;
    return (
        <span className="flex size-11 shrink-0 items-center justify-center rounded-2xl bg-linear-to-br from-primary/20 to-primary/5 text-primary ring-1 ring-primary/15">
            <item.icon className="size-5" />
        </span>
    );
}
