'use client';

import Link from 'next/link';
import type { LucideIcon } from 'lucide-react';
import { cn } from '@/lib/utils';

export type TabNavItem = {
    href: string;
    label: string;
    icon?: LucideIcon;
    /** Icon colour class (e.g. text-success). */
    tone?: string;
    active: boolean;
    count?: number;
};

/** Underlined section tabs with icons, shared by list pages. */
export function TabNav({
    items,
    label,
    className,
}: {
    items: TabNavItem[];
    label: string;
    className?: string;
}) {
    return (
        <nav aria-label={label} className={cn('border-b', className)}>
            <ul className="-mb-px flex gap-1 overflow-x-auto">
                {items.map((i) => (
                    <li key={i.href} className="shrink-0">
                        <Link
                            href={i.href}
                            aria-current={i.active ? 'page' : undefined}
                            className={cn(
                                'group relative flex items-center gap-2 px-3 pt-2 pb-3 text-sm whitespace-nowrap text-muted-foreground transition-colors outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring',
                                i.active && 'font-medium text-foreground',
                            )}
                        >
                            {i.icon && (
                                <i.icon
                                    className={cn(
                                        'size-4 transition-transform group-hover:scale-110',
                                        i.active
                                            ? (i.tone ?? 'text-primary')
                                            : i.tone && 'opacity-80',
                                    )}
                                />
                            )}
                            {i.label}
                            {i.count !== undefined && (
                                <span className="rounded-full bg-muted px-1.5 text-[11px] text-muted-foreground tabular-nums">
                                    {i.count}
                                </span>
                            )}
                            <span
                                className={cn(
                                    'absolute inset-x-2 -bottom-px h-0.5 rounded-full bg-primary transition-opacity',
                                    i.active ? 'opacity-100' : 'opacity-0',
                                )}
                            />
                        </Link>
                    </li>
                ))}
            </ul>
        </nav>
    );
}
