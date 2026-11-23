'use client';

import { Bell, Inbox, Megaphone, Search, Users } from 'lucide-react';
import { useT } from '@/i18n/client';
import type { Theme } from '@/lib/types';
import { cn } from '@/lib/utils';
import { themeCss } from '@/theme/css';

/**
 * A miniature panel rendered with a draft theme. The theme's variables are
 * scoped to this element, so the rest of the page keeps the saved theme.
 */
export function ThemePreview({
    theme,
    scheme,
    id = 'theme-preview',
    compact,
}: {
    theme: Theme;
    scheme: 'light' | 'dark';
    id?: string;
    compact?: boolean;
}) {
    const t = useT();
    const css = themeCss(theme, { light: `#${id}`, dark: `#${id}.dark` });
    const s = (k: Parameters<typeof t>[0]) => t(k);
    return (
        <>
            <style dangerouslySetInnerHTML={{ __html: css }} />
            <div
                id={id}
                className={cn(
                    scheme === 'dark' && 'dark',
                    'overflow-hidden rounded-[calc(var(--radius)*1.4)] border border-border bg-background text-foreground shadow-lg transition-colors',
                )}
                style={{ fontFamily: 'var(--app-font)' }}
            >
                <div className="flex">
                    <aside className="hidden w-36 shrink-0 flex-col gap-1 border-r border-sidebar-border bg-sidebar p-2 text-sidebar-foreground sm:flex">
                        <div className="mb-2 flex items-center gap-1.5 px-1">
                            <span className="size-5 rounded-[calc(var(--radius)*0.6)] bg-sidebar-primary" />
                            <span className="text-xs font-semibold">
                                Messora
                            </span>
                        </div>
                        {[
                            [Inbox, 'nav.inbox'],
                            [Users, 'nav.contacts'],
                            [Megaphone, 'nav.campaigns'],
                        ].map(([Icon, k], i) => {
                            const I = Icon as typeof Inbox;
                            return (
                                <span
                                    key={k as string}
                                    className={cn(
                                        'flex items-center gap-1.5 rounded-[calc(var(--radius)*0.6)] px-1.5 py-1 text-[11px]',
                                        i === 2 &&
                                            'bg-sidebar-accent font-medium text-sidebar-accent-foreground',
                                    )}
                                >
                                    <I className="size-3" />
                                    {s(k as Parameters<typeof t>[0])}
                                </span>
                            );
                        })}
                    </aside>
                    <div
                        className={cn(
                            'min-w-0 flex-1 space-y-3',
                            compact ? 'p-3' : 'p-4',
                        )}
                    >
                        <div className="flex items-center gap-2">
                            <div className="flex h-7 flex-1 items-center gap-1.5 rounded-[calc(var(--radius)*0.8)] border border-input px-2 text-[11px] text-muted-foreground">
                                <Search className="size-3" />
                                {s('appearance.previewSample.input')}
                            </div>
                            <span className="flex size-7 items-center justify-center rounded-[calc(var(--radius)*0.8)] bg-secondary text-secondary-foreground">
                                <Bell className="size-3" />
                            </span>
                        </div>
                        <div className="space-y-2 rounded-[var(--radius)] border border-border bg-card p-3 text-card-foreground shadow-xs">
                            <div className="flex items-center justify-between gap-2">
                                <span className="text-sm font-semibold">
                                    {s('appearance.previewSample.heading')}
                                </span>
                                <span className="rounded-full bg-success/15 px-2 py-0.5 text-[10px] font-medium text-success">
                                    {s('appearance.previewSample.badge')}
                                </span>
                            </div>
                            <p className="text-[11px] text-muted-foreground">
                                {s('appearance.previewSample.body')}
                            </p>
                            <div className="flex h-10 items-end gap-1">
                                {[40, 65, 50, 85, 70, 95, 60].map((h, i) => (
                                    <span
                                        key={i}
                                        className="flex-1 rounded-t-[3px]"
                                        style={{
                                            height: `${h}%`,
                                            background: `var(--chart-${(i % 5) + 1})`,
                                        }}
                                    />
                                ))}
                            </div>
                            <div className="flex gap-2 pt-1">
                                <span className="rounded-[calc(var(--radius)*0.8)] bg-primary px-3 py-1.5 text-[11px] font-medium text-primary-foreground">
                                    {s('appearance.previewSample.button')}
                                </span>
                                <span className="rounded-[calc(var(--radius)*0.8)] border border-border px-3 py-1.5 text-[11px] font-medium">
                                    {s('appearance.previewSample.secondary')}
                                </span>
                                <span className="ml-auto self-center rounded-[calc(var(--radius)*0.6)] bg-accent px-2 py-1 text-[10px] text-accent-foreground">
                                    98%
                                </span>
                            </div>
                        </div>
                        <div className="flex gap-1.5">
                            <span className="h-1.5 flex-1 rounded-full bg-destructive/80" />
                            <span className="h-1.5 flex-1 rounded-full bg-warning/80" />
                            <span className="h-1.5 flex-1 rounded-full bg-success/80" />
                        </div>
                    </div>
                </div>
            </div>
        </>
    );
}
