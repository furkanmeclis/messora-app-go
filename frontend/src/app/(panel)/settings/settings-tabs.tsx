'use client';

import Link from 'next/link';
import { usePathname } from 'next/navigation';
import {
    AppWindow,
    KeyRound,
    Mail,
    MessageSquareOff,
    Palette,
    ReceiptText,
    ShieldAlert,
    ShieldCheck,
    SlidersHorizontal,
    Sparkles,
    Webhook,
} from 'lucide-react';
import { cn } from '@/lib/utils';

const ICONS = {
    general: SlidersHorizontal,
    appearance: Palette,
    pwa: AppWindow,
    email: Mail,
    consent: MessageSquareOff,
    protection: ShieldAlert,
    security: ShieldCheck,
    license: KeyRound,
    webhooks: Webhook,
    metaRates: ReceiptText,
    ai: Sparkles,
} as const;

export function SettingsTabs({
    tabs,
}: {
    tabs: { href: string; label: string; icon: string }[];
}) {
    const pathname = usePathname();
    const active = tabs
        .filter((t) =>
            t.href === '/settings'
                ? pathname === '/settings'
                : pathname.startsWith(t.href),
        )
        .sort((a, b) => b.href.length - a.href.length)[0]?.href;
    return (
        <nav className="-mx-1 flex gap-1 overflow-x-auto border-b px-1">
            {tabs.map((t) => {
                const Icon = ICONS[t.icon as keyof typeof ICONS];
                const on = t.href === active;
                return (
                    <Link
                        key={t.href}
                        href={t.href}
                        aria-current={on ? 'page' : undefined}
                        className={cn(
                            'relative flex items-center gap-2 px-3 pt-2 pb-3 text-sm font-medium whitespace-nowrap text-muted-foreground transition-colors hover:text-foreground',
                            on && 'text-foreground',
                        )}
                    >
                        {Icon && <Icon className="size-4" />}
                        {t.label}
                        <span
                            className={cn(
                                'absolute inset-x-2 -bottom-px h-0.5 rounded-full bg-primary transition-opacity',
                                on ? 'opacity-100' : 'opacity-0',
                            )}
                        />
                    </Link>
                );
            })}
        </nav>
    );
}
