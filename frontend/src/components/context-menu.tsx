'use client';

import {
    createContext,
    useCallback,
    useContext,
    useEffect,
    useId,
    useLayoutEffect,
    useMemo,
    useRef,
    useState,
} from 'react';
import { useRouter } from 'next/navigation';
import { useTheme } from 'next-themes';
import { toast } from 'sonner';
import {
    ArrowLeft,
    ArrowRight,
    Command,
    Copy,
    ExternalLink,
    Image as ImageIcon,
    Keyboard,
    Link2,
    RotateCw,
    Search,
    SunMoon,
    type LucideIcon,
} from 'lucide-react';
import { useT } from '@/i18n/client';
import { cn } from '@/lib/utils';

export type MenuItem = {
    id: string;
    label: string;
    icon?: LucideIcon;
    shortcut?: string;
    danger?: boolean;
    disabled?: boolean;
    run: () => void;
};
/** Items of one region; groups are separated by a line. */
export type MenuGroup = MenuItem[];
type Provider = (target: HTMLElement) => MenuGroup[] | null | undefined;

type Ctx = { register: (id: string, fn: Provider) => () => void };
const MenuContext = createContext<Ctx | null>(null);

/** Fields keep the browser's own menu (paste, spelling, …). */
function editable(el: HTMLElement) {
    return (
        el.isContentEditable ||
        !!el.closest(
            'input, textarea, select, [contenteditable="true"], [data-native-context]',
        )
    );
}

async function copy(text: string, done: string) {
    try {
        await navigator.clipboard.writeText(text);
        toast.success(done);
    } catch {
        // clipboard refused: nothing to do
    }
}

/**
 * The panel's own right-click menu. Regions add items with
 * useContextMenu(); links, selected text and images get standard items;
 * every menu ends with navigation and panel actions. Shift + right-click
 * and text fields keep the browser's menu.
 */
export function ContextMenuProvider({
    children,
}: {
    children: React.ReactNode;
}) {
    const t = useT();
    const router = useRouter();
    const { resolvedTheme, setTheme } = useTheme();
    const providers = useRef(new Map<string, Provider>());
    const [menu, setMenu] = useState<{
        x: number;
        y: number;
        groups: MenuGroup[];
    } | null>(null);
    const [active, setActive] = useState(0);
    const box = useRef<HTMLDivElement>(null);
    const [pos, setPos] = useState<{ left: number; top: number } | null>(null);

    const register = useCallback((id: string, fn: Provider) => {
        providers.current.set(id, fn);
        return () => {
            providers.current.delete(id);
        };
    }, []);

    useEffect(() => {
        const onContext = (e: MouseEvent) => {
            const target = e.target as HTMLElement | null;
            if (!target || e.shiftKey || editable(target)) return;
            const groups: MenuGroup[] = [];

            // Regions, innermost first.
            let el: HTMLElement | null = target;
            while (el) {
                const id = el.dataset.contextMenu;
                const fn = id ? providers.current.get(id) : undefined;
                if (fn)
                    for (const g of fn(target) ?? [])
                        if (g.length) groups.push(g);
                el = el.parentElement;
            }

            const link = target.closest<HTMLAnchorElement>('a[href]');
            if (link) {
                const href = link.getAttribute('href')!;
                const internal = href.startsWith('/');
                const g: MenuGroup = [
                    {
                        id: 'open',
                        label: t('contextMenu.open'),
                        icon: ExternalLink,
                        run: () =>
                            internal
                                ? router.push(href)
                                : window.open(href, '_blank', 'noopener'),
                    },
                    {
                        id: 'tab',
                        label: t('contextMenu.openNewTab'),
                        icon: ExternalLink,
                        run: () => window.open(link.href, '_blank', 'noopener'),
                    },
                ];
                g.push({
                    id: 'copy-link',
                    label: t('contextMenu.copyLink'),
                    icon: Link2,
                    run: () => copy(link.href, t('contextMenu.copied')),
                });
                groups.push(g);
            }

            const img = target.closest<HTMLImageElement>('img[src]');
            if (img) {
                groups.push([
                    {
                        id: 'img',
                        label: t('contextMenu.openImage'),
                        icon: ImageIcon,
                        run: () => window.open(img.src, '_blank', 'noopener'),
                    },
                ]);
            }

            const text = window.getSelection()?.toString().trim() ?? '';
            if (text) {
                const short = text.length > 24 ? `${text.slice(0, 24)}…` : text;
                groups.push([
                    {
                        id: 'copy',
                        label: t('contextMenu.copy'),
                        icon: Copy,
                        shortcut: '⌘C',
                        run: () => copy(text, t('contextMenu.copied')),
                    },
                    {
                        id: 'search',
                        label: t('contextMenu.searchContacts', { text: short }),
                        icon: Search,
                        run: () =>
                            router.push(
                                `/contacts?q=${encodeURIComponent(text.slice(0, 100))}`,
                            ),
                    },
                ]);
            }

            const general: MenuGroup = [
                {
                    id: 'back',
                    label: t('contextMenu.back'),
                    icon: ArrowLeft,
                    run: () => router.back(),
                },
                {
                    id: 'forward',
                    label: t('contextMenu.forward'),
                    icon: ArrowRight,
                    run: () => router.forward(),
                },
                {
                    id: 'reload',
                    label: t('contextMenu.reload'),
                    icon: RotateCw,
                    run: () => router.refresh(),
                },
            ];
            const panel: MenuGroup = [
                {
                    id: 'palette',
                    label: t('contextMenu.palette'),
                    icon: Command,
                    shortcut: '⌘K',
                    run: () =>
                        window.dispatchEvent(new Event('messora:palette-open')),
                },
                {
                    id: 'shortcuts',
                    label: t('contextMenu.shortcuts'),
                    icon: Keyboard,
                    shortcut: '?',
                    run: () =>
                        window.dispatchEvent(
                            new Event('messora:shortcuts-open'),
                        ),
                },
            ];
            panel.push({
                id: 'theme',
                label: t('contextMenu.theme'),
                icon: SunMoon,
                run: () =>
                    setTheme(resolvedTheme === 'dark' ? 'light' : 'dark'),
            });
            groups.push(general, panel);

            e.preventDefault();
            setActive(-1);
            setPos(null);
            setMenu({ x: e.clientX, y: e.clientY, groups });
        };
        document.addEventListener('contextmenu', onContext);
        return () => document.removeEventListener('contextmenu', onContext);
    }, [t, router, resolvedTheme, setTheme]);

    const flat = useMemo(
        () => menu?.groups.flat().filter((i) => !i.disabled) ?? [],
        [menu],
    );
    const close = useCallback(() => setMenu(null), []);

    // Keep the menu inside the viewport.
    useLayoutEffect(() => {
        if (!menu || !box.current) return;
        const r = box.current.getBoundingClientRect();
        setPos({
            left: Math.max(
                4,
                Math.min(menu.x, window.innerWidth - r.width - 4),
            ),
            top: Math.max(
                4,
                Math.min(menu.y, window.innerHeight - r.height - 4),
            ),
        });
        box.current.focus();
    }, [menu]);

    useEffect(() => {
        if (!menu) return;
        const onDown = (e: PointerEvent) => {
            if (!box.current?.contains(e.target as Node)) close();
        };
        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape') close();
        };
        window.addEventListener('pointerdown', onDown, true);
        window.addEventListener('keydown', onKey);
        window.addEventListener('blur', close);
        window.addEventListener('resize', close);
        window.addEventListener('scroll', close, true);
        return () => {
            window.removeEventListener('pointerdown', onDown, true);
            window.removeEventListener('keydown', onKey);
            window.removeEventListener('blur', close);
            window.removeEventListener('resize', close);
            window.removeEventListener('scroll', close, true);
        };
    }, [menu, close]);

    let index = -1;
    return (
        <MenuContext.Provider value={{ register }}>
            {children}
            {menu && (
                <div
                    ref={box}
                    role="menu"
                    tabIndex={-1}
                    aria-label={t('contextMenu.nativeHint')}
                    onKeyDown={(e) => {
                        if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
                            e.preventDefault();
                            const d = e.key === 'ArrowDown' ? 1 : -1;
                            setActive(
                                (a) => (a + d + flat.length) % flat.length,
                            );
                        } else if (e.key === 'Enter' && flat[active]) {
                            e.preventDefault();
                            close();
                            flat[active]!.run();
                        }
                    }}
                    className="fixed z-[20000] min-w-56 animate-in rounded-xl border bg-popover/95 p-1 text-sm text-popover-foreground shadow-xl shadow-black/15 backdrop-blur-xl fade-in-0 outline-none zoom-in-95"
                    style={
                        pos ?? {
                            left: menu.x,
                            top: menu.y,
                            visibility: 'hidden',
                        }
                    }
                    onContextMenu={(e) => e.preventDefault()}
                >
                    {menu.groups.map((g, gi) => (
                        <div
                            key={gi}
                            className={cn(gi > 0 && 'mt-1 border-t pt-1')}
                        >
                            {g.map((item) => {
                                const i = item.disabled ? -2 : ++index;
                                return (
                                    <button
                                        key={item.id}
                                        type="button"
                                        role="menuitem"
                                        disabled={item.disabled}
                                        onMouseEnter={() => setActive(i)}
                                        onClick={() => {
                                            close();
                                            item.run();
                                        }}
                                        className={cn(
                                            'flex w-full items-center gap-2.5 rounded-lg px-2 py-1.5 text-left outline-none disabled:opacity-40',
                                            i === active &&
                                                'bg-accent text-accent-foreground',
                                            item.danger && 'text-destructive',
                                        )}
                                    >
                                        {item.icon ? (
                                            <item.icon className="size-4 shrink-0 text-muted-foreground" />
                                        ) : (
                                            <span className="size-4" />
                                        )}
                                        <span className="flex-1 truncate">
                                            {item.label}
                                        </span>
                                        {item.shortcut && (
                                            <span
                                                lang="en"
                                                className="text-xs text-muted-foreground"
                                            >
                                                {item.shortcut}
                                            </span>
                                        )}
                                    </button>
                                );
                            })}
                        </div>
                    ))}
                    <p className="mt-1 border-t px-2 pt-1.5 pb-0.5 text-[11px] text-muted-foreground">
                        {t('contextMenu.nativeHint')}
                    </p>
                </div>
            )}
        </MenuContext.Provider>
    );
}

/**
 * Adds items to the right-click menu of an element. Spread the returned
 * props on it: `<div {...useContextMenu(() => [[…]])}>`.
 */
export function useContextMenu(items: Provider) {
    const ctx = useContext(MenuContext);
    const id = useId();
    const fn = useRef(items);
    useEffect(() => {
        fn.current = items;
    });
    useEffect(
        () => ctx?.register(id, (target) => fn.current(target)),
        [ctx, id],
    );
    return { 'data-context-menu': id };
}
