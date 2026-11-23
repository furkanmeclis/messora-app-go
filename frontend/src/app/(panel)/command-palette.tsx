'use client';

import { useEffect, useMemo, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { useTheme } from 'next-themes';
import {
    CornerDownLeft,
    Megaphone,
    SunMoon,
    Upload,
    UserPlus,
    Workflow,
    Search,
    AppWindow,
    Download,
    type LucideIcon,
} from 'lucide-react';
import { Dialog, DialogContent, DialogTitle } from '@/components/ui/dialog';
import { useT } from '@/i18n/client';
import { useDesktop } from '@/components/desktop/desktop-context';
import { usePwa } from '@/components/pwa';
import { visibleNav } from '@/lib/navigation';
import { cn } from '@/lib/utils';

type Item = {
    id: string;
    label: string;
    group: string;
    icon: LucideIcon;
    run: () => void;
};

/** ⌘K / Ctrl+K: jump to any page or start a common task. */
export function CommandPalette({
    open,
    onOpenChange,
    permissions,
    restricted,
}: {
    open: boolean;
    onOpenChange: (o: boolean) => void;
    permissions: string[];
    restricted: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const { resolvedTheme, setTheme } = useTheme();
    const [query, setQuery] = useState('');
    const desktop = useDesktop();
    const pwa = usePwa();
    const [active, setActive] = useState(0);
    const list = useRef<HTMLDivElement>(null);

    const items = useMemo<Item[]>(() => {
        const go = (href: string) => () => router.push(href);
        const has = (p: string) => permissions.includes(p);
        const pages: Item[] = visibleNav(permissions, restricted).map((n) => ({
            id: n.id,
            label: t(n.label),
            group: t('topbar.goTo'),
            icon: n.icon,
            run: go(n.href),
        }));
        const actions: (Item | false)[] = [
            !restricted &&
                has('campaigns.create') && {
                    id: 'new-campaign',
                    label: t('topbar.newCampaign'),
                    group: t('topbar.actions'),
                    icon: Megaphone,
                    run: go('/campaigns/new'),
                },
            !restricted &&
                has('contacts.import') && {
                    id: 'import',
                    label: t('topbar.importContacts'),
                    group: t('topbar.actions'),
                    icon: Upload,
                    run: go('/contacts/import'),
                },
            !restricted &&
                has('automations.manage') && {
                    id: 'new-automation',
                    label: t('topbar.newAutomation'),
                    group: t('topbar.actions'),
                    icon: Workflow,
                    run: go('/automations/new'),
                },
            !restricted &&
                has('users.manage') && {
                    id: 'invite',
                    label: t('topbar.inviteUser'),
                    group: t('topbar.actions'),
                    icon: UserPlus,
                    run: go('/users'),
                },
            !restricted &&
                desktop.available && {
                    id: 'desktop',
                    label: t('desktop.enter'),
                    group: t('topbar.actions'),
                    icon: AppWindow,
                    run: desktop.enter,
                },
            pwa.canInstall && {
                id: 'install',
                label: t('pwa.install'),
                group: t('topbar.actions'),
                icon: Download,
                run: () => void pwa.install(),
            },
            {
                id: 'theme',
                label: t('topbar.toggleTheme'),
                group: t('topbar.actions'),
                icon: SunMoon,
                run: () =>
                    setTheme(resolvedTheme === 'dark' ? 'light' : 'dark'),
            },
        ];
        return [...pages, ...actions.filter((a): a is Item => !!a)];
    }, [
        permissions,
        restricted,
        router,
        t,
        resolvedTheme,
        setTheme,
        desktop,
        pwa,
    ]);

    const filtered = useMemo(() => {
        const q = query.trim().toLocaleLowerCase();
        return q
            ? items.filter((i) => i.label.toLocaleLowerCase().includes(q))
            : items;
    }, [items, query]);

    useEffect(() => {
        list.current
            ?.querySelector(`[data-index="${active}"]`)
            ?.scrollIntoView({ block: 'nearest' });
    }, [active]);

    const run = (i: Item | undefined) => {
        if (!i) return;
        onOpenChange(false);
        i.run();
    };

    let lastGroup = '';
    return (
        <Dialog
            open={open}
            onOpenChange={(o) => {
                onOpenChange(o);
                if (o) {
                    setQuery('');
                    setActive(0);
                }
            }}
        >
            <DialogContent
                showCloseButton={false}
                className="top-[18%] translate-y-0 gap-0 overflow-hidden p-0 sm:max-w-xl"
            >
                <DialogTitle className="sr-only">
                    {t('topbar.search')}
                </DialogTitle>
                <div className="flex items-center gap-3 border-b px-4">
                    <Search className="size-4 text-muted-foreground" />
                    <input
                        autoFocus
                        value={query}
                        placeholder={t('topbar.search')}
                        className="h-12 w-full bg-transparent text-sm outline-none placeholder:text-muted-foreground"
                        onChange={(e) => {
                            setQuery(e.target.value);
                            setActive(0);
                        }}
                        onKeyDown={(e) => {
                            if (e.key === 'ArrowDown') {
                                e.preventDefault();
                                setActive((a) =>
                                    Math.min(filtered.length - 1, a + 1),
                                );
                            } else if (e.key === 'ArrowUp') {
                                e.preventDefault();
                                setActive((a) => Math.max(0, a - 1));
                            } else if (e.key === 'Enter') {
                                e.preventDefault();
                                run(filtered[active]);
                            }
                        }}
                    />
                    <kbd className="rounded border bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
                        Esc
                    </kbd>
                </div>
                <div
                    ref={list}
                    role="listbox"
                    className="max-h-[50vh] overflow-y-auto p-2"
                >
                    {filtered.length === 0 && (
                        <p className="py-10 text-center text-sm text-muted-foreground">
                            {t('common.noResults')}
                        </p>
                    )}
                    {filtered.map((item, i) => {
                        const heading = item.group !== lastGroup;
                        lastGroup = item.group;
                        return (
                            <div key={item.id}>
                                {heading && (
                                    <p className="px-2 pt-2 pb-1 text-[11px] font-medium tracking-wide text-muted-foreground uppercase">
                                        {item.group}
                                    </p>
                                )}
                                <div
                                    role="option"
                                    aria-selected={i === active}
                                    data-index={i}
                                    onMouseEnter={() => setActive(i)}
                                    onClick={() => run(item)}
                                    className={cn(
                                        'flex cursor-pointer items-center gap-3 rounded-lg px-2 py-2 text-sm',
                                        i === active &&
                                            'bg-accent text-accent-foreground',
                                    )}
                                >
                                    <span className="flex size-7 items-center justify-center rounded-md border bg-background">
                                        <item.icon className="size-3.5" />
                                    </span>
                                    {item.label}
                                    {i === active && (
                                        <CornerDownLeft className="ml-auto size-3.5 text-muted-foreground" />
                                    )}
                                </div>
                            </div>
                        );
                    })}
                </div>
            </DialogContent>
        </Dialog>
    );
}
