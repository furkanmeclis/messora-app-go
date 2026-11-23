'use client';

import { useEffect, useState } from 'react';
import Link from 'next/link';
import { usePathname } from 'next/navigation';
import { useTheme } from 'next-themes';
import {
    Bell,
    Inbox,
    Megaphone,
    MessageCircle,
    Moon,
    Search,
    Send,
    Sun,
    Activity,
    AppWindow,
} from 'lucide-react';
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from '@/components/ui/popover';
import { Separator } from '@/components/ui/separator';
import { SidebarTrigger } from '@/components/ui/sidebar';
import {
    Tooltip,
    TooltipContent,
    TooltipTrigger,
} from '@/components/ui/tooltip';
import {
    useRealtimeEvents,
    useRealtimeState,
    type RealtimeEvent,
} from '@/components/realtime';
import { useT } from '@/i18n/client';
import type { Translate } from '@/i18n/translate';
import { activeNav } from '@/lib/navigation';
import { NUMBER_LOCALE } from '@/lib/format';
import { cn } from '@/lib/utils';
import { CommandPalette } from './command-palette';
import { useDesktop } from '@/components/desktop/desktop-context';
import { campaignSummaryAction } from './topbar-actions';

export type RunningCampaign = {
    id: string;
    name: string;
    total: number;
    counts: Record<string, number>;
};

type FeedItem = {
    key: string;
    at: number;
    text: string;
    href?: string;
    icon: 'in' | 'out' | 'campaign' | 'status';
};

const n = (v: number) => v.toLocaleString(NUMBER_LOCALE);

/** Recipients that reached a final state (for the progress bar). */
function processed(c: Record<string, number>) {
    return [
        'sent',
        'delivered',
        'read',
        'failed',
        'unknown',
        'recipients_skipped',
    ].reduce((s, k) => s + (c[k] ?? 0), 0);
}

function describe(
    t: Translate,
    e: RealtimeEvent,
): Omit<FeedItem, 'key' | 'at'> | null {
    const word = (group: 'statuses' | 'kinds', v: unknown) => {
        const s = String(v ?? '');
        const k = `topbar.${group}.${s}` as Parameters<Translate>[0];
        const r = t(k);
        return r === k ? s : r;
    };
    switch (e.type) {
        case 'message.created':
            return e.direction === 'inbound'
                ? {
                      text: t('topbar.events.inbound', {
                          kind: word('kinds', e.message_type),
                      }),
                      icon: 'in',
                      href: e.conversation_id
                          ? `/inbox?c=${String(e.conversation_id)}`
                          : '/inbox',
                  }
                : {
                      text: t('topbar.events.outbound', {
                          kind: word('kinds', e.message_type),
                      }),
                      icon: 'out',
                  };
        case 'message.status':
            if (e.status !== 'failed') return null; // only what needs attention
            return {
                text: t('topbar.events.status', {
                    status: word('statuses', e.status),
                }),
                icon: 'status',
            };
        case 'campaign.updated':
            return {
                text: t('topbar.events.campaign', {
                    status: word('statuses', e.status),
                }),
                icon: 'campaign',
                href: `/campaigns/${String(e.campaign_id)}`,
            };
        default:
            return null;
    }
}

function ago(t: Translate, at: number, now: number) {
    const m = Math.floor((now - at) / 60000);
    return m < 1 ? t('topbar.justNow') : t('topbar.minutesAgo', { count: m });
}

/**
 * The panel's top bar: where you are, a command palette, and live state
 * straight from the realtime connection (unread conversations, campaigns
 * sending right now, recent activity).
 */
export function Topbar({
    permissions,
    restricted,
    initialUnread,
    initialRunning,
}: {
    permissions: string[];
    restricted: boolean;
    initialUnread: string[];
    initialRunning: RunningCampaign[];
}) {
    const t = useT();
    const pathname = usePathname();
    const state = useRealtimeState();
    const { resolvedTheme, setTheme } = useTheme();
    const [palette, setPalette] = useState(false);
    const desktop = useDesktop();
    const [unread, setUnread] = useState(() => new Set(initialUnread));
    const [running, setRunning] = useState(
        () => new Map(initialRunning.map((c) => [c.id, c])),
    );
    const [feed, setFeed] = useState<FeedItem[]>([]);
    const [seen, setSeen] = useState(0);
    const [pulse, setPulse] = useState(0);
    const [now, setNow] = useState(() => Date.now());
    const section = activeNav(pathname);
    const canInbox =
        permissions.includes('conversations.read') ||
        permissions.includes('conversations.read_all');
    const canCampaigns = permissions.includes('campaigns.read');

    useEffect(() => {
        const onKey = (e: KeyboardEvent) => {
            if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
                e.preventDefault();
                setPalette((o) => !o);
            }
        };
        window.addEventListener('keydown', onKey);
        const tick = setInterval(() => setNow(Date.now()), 30_000);
        return () => {
            window.removeEventListener('keydown', onKey);
            clearInterval(tick);
        };
    }, []);

    useRealtimeEvents((_channel, e) => {
        if (e.type === 'conversation.updated') {
            const id = String(e.conversation_id);
            setUnread((prev) => {
                const has = prev.has(id);
                const want =
                    Number(e.unread_count ?? 0) > 0 && e.status !== 'closed';
                if (has === want) return prev;
                const next = new Set(prev);
                if (want) next.add(id);
                else next.delete(id);
                return next;
            });
            return;
        }
        if (e.type === 'campaign.updated') {
            const id = String(e.campaign_id);
            if (e.status === 'running' && !running.get(id)?.name) {
                void campaignSummaryAction(id).then((c) => {
                    if (!c) return;
                    setRunning((prev) => {
                        const cur = prev.get(id);
                        if (!cur) return prev;
                        return new Map(prev).set(id, { ...cur, ...c });
                    });
                });
            }
            setRunning((prev) => {
                const next = new Map(prev);
                const cur = next.get(id);
                if (e.status === 'running') {
                    next.set(id, {
                        id,
                        name: cur?.name ?? '',
                        total: cur?.total ?? 0,
                        counts:
                            (e.counts as Record<string, number>) ??
                            cur?.counts ??
                            {},
                    });
                } else {
                    next.delete(id);
                }
                return next;
            });
        }
        if (e.type === 'message.created' && e.direction === 'inbound') {
            setPulse((p) => p + 1);
        }
        const d = describe(t, e);
        if (d) {
            setFeed((f) =>
                [
                    {
                        ...d,
                        key: `${e.type}:${String(e.message_id ?? e.campaign_id)}:${Date.now()}`,
                        at: Date.now(),
                    },
                    ...f,
                ].slice(0, 30),
            );
        }
    });

    const unreadCount = unread.size;
    const campaigns = [...running.values()];
    const fresh = feed.length - seen;
    const stateLabel = {
        connected: t('topbar.live'),
        connecting: t('topbar.connecting'),
        disconnected: t('topbar.offline'),
    }[state];

    const iconButton =
        'relative flex size-9 items-center justify-center rounded-xl text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground focus-visible:ring-3 focus-visible:ring-ring/50 outline-none';

    return (
        <header
            data-chrome
            className="sticky top-0 z-30 flex h-14 shrink-0 items-center gap-2 border-b bg-background/75 px-3 backdrop-blur-xl md:px-4"
        >
            <SidebarTrigger className="-ml-1" />
            <Separator orientation="vertical" className="mr-1 h-4" />
            {section && (
                <div className="flex min-w-0 items-center gap-2 text-sm font-medium">
                    <section.icon className="size-4 shrink-0 text-primary" />
                    <span className="truncate">{t(section.label)}</span>
                </div>
            )}

            <button
                type="button"
                onClick={() => setPalette(true)}
                className="mx-auto hidden h-9 w-full max-w-sm items-center gap-2 rounded-xl border bg-muted/40 px-3 text-sm text-muted-foreground transition-colors hover:bg-muted md:flex"
            >
                <Search className="size-4" />
                <span className="truncate">{t('topbar.search')}</span>
                <kbd className="ml-auto rounded-md border bg-background px-1.5 text-[10px] font-medium">
                    ⌘K
                </kbd>
            </button>

            <div className="ml-auto flex items-center gap-1 md:ml-0">
                <button
                    type="button"
                    aria-label={t('topbar.search')}
                    onClick={() => setPalette(true)}
                    className={cn(iconButton, 'md:hidden')}
                >
                    <Search className="size-4" />
                </button>

                {!restricted && (
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <span
                                className={cn(
                                    'mr-1 hidden items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium sm:flex',
                                    state === 'connected' &&
                                        'border-success/30 bg-success/10 text-success',
                                    state === 'connecting' &&
                                        'border-warning/30 bg-warning/10 text-warning',
                                    state === 'disconnected' &&
                                        'text-muted-foreground',
                                )}
                            >
                                <span className="relative flex size-2">
                                    {state === 'connected' && (
                                        <span className="absolute inline-flex size-full animate-ping rounded-full bg-success opacity-60" />
                                    )}
                                    <span
                                        className={cn(
                                            'relative inline-flex size-2 rounded-full',
                                            state === 'connected'
                                                ? 'bg-success'
                                                : state === 'connecting'
                                                  ? 'bg-warning'
                                                  : 'bg-muted-foreground',
                                        )}
                                    />
                                </span>
                                {stateLabel}
                            </span>
                        </TooltipTrigger>
                        <TooltipContent>{t('topbar.liveHint')}</TooltipContent>
                    </Tooltip>
                )}

                {canCampaigns && !restricted && (
                    <Popover>
                        <PopoverTrigger asChild>
                            <button
                                type="button"
                                aria-label={
                                    campaigns.length
                                        ? t('topbar.campaignsRunning', {
                                              count: campaigns.length,
                                          })
                                        : t('topbar.campaignsIdle')
                                }
                                className={cn(
                                    iconButton,
                                    campaigns.length && 'text-primary',
                                )}
                            >
                                <Megaphone
                                    className={cn(
                                        'size-4',
                                        campaigns.length && 'animate-pulse',
                                    )}
                                />
                                {campaigns.length > 0 && (
                                    <span className="absolute -top-0.5 -right-0.5 flex size-4 items-center justify-center rounded-full bg-primary text-[10px] font-semibold text-primary-foreground">
                                        {campaigns.length}
                                    </span>
                                )}
                            </button>
                        </PopoverTrigger>
                        <PopoverContent align="end" className="w-80 space-y-3">
                            <p className="text-sm font-medium">
                                {campaigns.length
                                    ? t('topbar.campaignsRunning', {
                                          count: campaigns.length,
                                      })
                                    : t('topbar.campaignsIdle')}
                            </p>
                            {campaigns.map((c) => {
                                const done = processed(c.counts);
                                const pct = c.total
                                    ? Math.min(
                                          100,
                                          Math.round((done / c.total) * 100),
                                      )
                                    : 0;
                                return (
                                    <Link
                                        key={c.id}
                                        href={`/campaigns/${c.id}`}
                                        className="block space-y-1.5 rounded-lg border p-2.5 transition-colors hover:bg-accent/50"
                                    >
                                        <span className="flex items-center justify-between gap-2 text-sm">
                                            <span className="truncate font-medium">
                                                {c.name || '—'}
                                            </span>
                                            <span className="text-xs text-muted-foreground tabular-nums">
                                                {pct}%
                                            </span>
                                        </span>
                                        <span className="block h-1.5 overflow-hidden rounded-full bg-muted">
                                            <span
                                                className="block h-full rounded-full bg-primary transition-[width] duration-700"
                                                style={{ width: `${pct}%` }}
                                            />
                                        </span>
                                        {c.total > 0 && (
                                            <span className="block text-xs text-muted-foreground tabular-nums">
                                                {t('topbar.progress', {
                                                    done: n(done),
                                                    total: n(c.total),
                                                })}
                                            </span>
                                        )}
                                    </Link>
                                );
                            })}
                        </PopoverContent>
                    </Popover>
                )}

                {canInbox && !restricted && (
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <Link
                                href="/inbox"
                                aria-label={
                                    unreadCount
                                        ? t('topbar.unread', {
                                              count: unreadCount,
                                          })
                                        : t('topbar.noUnread')
                                }
                                className={iconButton}
                            >
                                <Inbox
                                    key={pulse}
                                    className={cn(
                                        'size-4',
                                        pulse > 0 &&
                                            'animate-in duration-500 zoom-in-50',
                                    )}
                                />
                                {unreadCount > 0 && (
                                    <span className="absolute -top-0.5 -right-0.5 flex h-4 min-w-4 items-center justify-center rounded-full bg-destructive px-1 text-[10px] font-semibold text-white">
                                        {unreadCount > 99 ? '99+' : unreadCount}
                                    </span>
                                )}
                            </Link>
                        </TooltipTrigger>
                        <TooltipContent>
                            {unreadCount
                                ? t('topbar.unread', { count: unreadCount })
                                : t('topbar.noUnread')}
                        </TooltipContent>
                    </Tooltip>
                )}

                {!restricted && (
                    <Popover onOpenChange={(o) => o && setSeen(feed.length)}>
                        <PopoverTrigger asChild>
                            <button
                                type="button"
                                aria-label={t('topbar.activity')}
                                className={iconButton}
                            >
                                <Bell className="size-4" />
                                {fresh > 0 && (
                                    <span className="absolute top-1.5 right-1.5 size-2 rounded-full bg-primary ring-2 ring-background" />
                                )}
                            </button>
                        </PopoverTrigger>
                        <PopoverContent align="end" className="w-80 p-0">
                            <div className="flex items-center justify-between border-b px-3 py-2.5">
                                <span className="flex items-center gap-2 text-sm font-medium">
                                    <Activity className="size-4 text-primary" />
                                    {t('topbar.activity')}
                                </span>
                            </div>
                            <ul className="max-h-80 overflow-y-auto p-1">
                                {feed.length === 0 && (
                                    <li className="px-3 py-8 text-center text-xs text-muted-foreground">
                                        {t('topbar.activityEmpty')}
                                    </li>
                                )}
                                {feed.map((f) => {
                                    const Icon = {
                                        in: MessageCircle,
                                        out: Send,
                                        campaign: Megaphone,
                                        status: Bell,
                                    }[f.icon];
                                    const body = (
                                        <>
                                            <span
                                                className={cn(
                                                    'flex size-7 shrink-0 items-center justify-center rounded-lg',
                                                    f.icon === 'in' &&
                                                        'bg-success/15 text-success',
                                                    f.icon === 'out' &&
                                                        'bg-muted text-muted-foreground',
                                                    f.icon === 'campaign' &&
                                                        'bg-primary/15 text-primary',
                                                    f.icon === 'status' &&
                                                        'bg-destructive/15 text-destructive',
                                                )}
                                            >
                                                <Icon className="size-3.5" />
                                            </span>
                                            <span className="min-w-0 flex-1 truncate">
                                                {f.text}
                                            </span>
                                            <span className="shrink-0 text-[11px] text-muted-foreground">
                                                {ago(t, f.at, now)}
                                            </span>
                                        </>
                                    );
                                    return (
                                        <li key={f.key}>
                                            {f.href ? (
                                                <Link
                                                    href={f.href}
                                                    className="flex items-center gap-2.5 rounded-lg px-2 py-2 text-sm hover:bg-accent"
                                                >
                                                    {body}
                                                </Link>
                                            ) : (
                                                <div className="flex items-center gap-2.5 px-2 py-2 text-sm">
                                                    {body}
                                                </div>
                                            )}
                                        </li>
                                    );
                                })}
                            </ul>
                        </PopoverContent>
                    </Popover>
                )}

                {desktop.available && !restricted && (
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <button
                                type="button"
                                aria-label={t('desktop.enter')}
                                onClick={desktop.enter}
                                className={iconButton}
                            >
                                <AppWindow className="size-4" />
                            </button>
                        </TooltipTrigger>
                        <TooltipContent className="max-w-64">
                            {t('desktop.enterHint')}
                        </TooltipContent>
                    </Tooltip>
                )}
                <button
                    type="button"
                    aria-label={t('topbar.toggleTheme')}
                    onClick={() =>
                        setTheme(resolvedTheme === 'dark' ? 'light' : 'dark')
                    }
                    className={iconButton}
                >
                    <Sun className="size-4 dark:hidden" />
                    <Moon className="hidden size-4 dark:block" />
                </button>
            </div>

            <CommandPalette
                open={palette}
                onOpenChange={setPalette}
                permissions={permissions}
                restricted={restricted}
            />
        </header>
    );
}
