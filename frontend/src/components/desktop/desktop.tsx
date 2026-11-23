'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { useRouter } from 'next/navigation';
import {
    Copy,
    ExternalLink,
    LayoutGrid,
    Layers,
    LogOut,
    Maximize2,
    Minus,
    PanelsTopLeft,
    RotateCw,
    Search,
    Square,
    X,
} from 'lucide-react';
import { BrandMark } from '@/components/brand';
import { useDisplayPrefs } from '@/components/display-preferences';
import { useRealtimeState } from '@/components/realtime';
import {
    Popover,
    PopoverContent,
    PopoverTrigger,
} from '@/components/ui/popover';
import {
    Tooltip,
    TooltipContent,
    TooltipTrigger,
} from '@/components/ui/tooltip';
import { useT } from '@/i18n/client';
import { activeNav, NAV, visibleNav, type NavItem } from '@/lib/navigation';
import { cn } from '@/lib/utils';
import { WALLPAPERS, WALLPAPER_SIZE } from '@/theme/wallpapers';
import type { Theme } from '@/lib/types';
import { DESKTOP_KEY_MESSAGE, useDesktop } from './desktop-context';

type Rect = { x: number; y: number; w: number; h: number };
type Win = Rect & {
    id: string;
    app: string;
    url: string;
    title: string;
    z: number;
    min: boolean;
    max: boolean;
    prev?: Rect;
};

const TASKBAR = 52;
const MIN_W = 420;
const MIN_H = 280;
const STORE = 'messora:desktop';

function load(): Win[] {
    try {
        const raw = localStorage.getItem(STORE);
        const list = raw ? (JSON.parse(raw) as Win[]) : [];
        return Array.isArray(list)
            ? list.filter(
                  (w) => typeof w.url === 'string' && w.url.startsWith('/'),
              )
            : [];
    } catch {
        return [];
    }
}

/** The frames of open windows (there is one desktop per page). */
const frames = new Map<string, HTMLIFrameElement>();

const appOf = (id: string): NavItem | undefined => NAV.find((n) => n.id === id);

export function Desktop({
    permissions,
    wallpaper,
    organization,
}: {
    permissions: string[];
    wallpaper: Theme['wallpaper'];
    organization: string;
}) {
    const t = useT();
    const router = useRouter();
    const { active, exit } = useDesktop();
    const prefs = useDisplayPrefs();
    const realtime = useRealtimeState();
    const apps = useMemo(() => visibleNav(permissions, false), [permissions]);
    const [wins, setWins] = useState<Win[]>([]);
    const [loaded, setLoaded] = useState(false);
    const [dragging, setDragging] = useState(false);
    const [selected, setSelected] = useState<string | null>(null);
    const [now, setNow] = useState(() => new Date());
    const [query, setQuery] = useState('');
    const [startOpen, setStartOpen] = useState(false);

    // Restore the last layout once (browser storage).
    useEffect(() => {
        if (!active || loaded) return;
        const saved = load();
        // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time restore
        setWins(saved);
        setLoaded(true);
    }, [active, loaded]);
    useEffect(() => {
        if (!loaded) return;
        try {
            localStorage.setItem(STORE, JSON.stringify(wins));
        } catch {
            // ignore
        }
    }, [wins, loaded]);
    useEffect(() => {
        const id = setInterval(() => setNow(new Date()), 15_000);
        return () => clearInterval(id);
    }, []);

    const area = () => ({
        w: window.innerWidth,
        h: window.innerHeight - TASKBAR,
    });
    const focused = useMemo(
        () => wins.filter((w) => !w.min).sort((a, b) => b.z - a.z)[0]?.id,
        [wins],
    );
    const update = (id: string, patch: Partial<Win>) =>
        setWins((ws) => ws.map((w) => (w.id === id ? { ...w, ...patch } : w)));
    const focus = useCallback((id: string) => {
        setWins((ws) => {
            const z = Math.max(0, ...ws.map((w) => w.z)) + 1;
            return ws.map((w) => (w.id === id ? { ...w, z, min: false } : w));
        });
    }, []);

    const open = (app: NavItem, url = app.href) => {
        const { w: aw, h: ah } = area();
        const n = wins.length;
        const w = Math.min(1100, Math.round(aw * 0.62));
        const h = Math.min(760, Math.round(ah * 0.78));
        const win: Win = {
            id: crypto.randomUUID(),
            app: app.id,
            url,
            title: t(app.label),
            x: Math.max(16, Math.round((aw - w) / 2) + ((n % 6) - 3) * 32),
            y: Math.max(16, Math.round((ah - h) / 3) + (n % 6) * 28),
            w,
            h,
            z: Math.max(0, ...wins.map((x) => x.z)) + 1,
            min: false,
            max: false,
        };
        setWins((ws) => [...ws, win]);
        setStartOpen(false);
    };
    const close = useCallback((id: string) => {
        frames.delete(id);
        setWins((ws) => ws.filter((w) => w.id !== id));
    }, []);
    const toggleMax = useCallback((id: string) => {
        setWins((ws) =>
            ws.map((w) => {
                if (w.id !== id) return w;
                if (w.max) return { ...w, max: false, ...(w.prev ?? {}) };
                return {
                    ...w,
                    max: true,
                    prev: { x: w.x, y: w.y, w: w.w, h: w.h },
                };
            }),
        );
    }, []);
    const minimize = useCallback((id: string) => update(id, { min: true }), []);

    const arrange = (mode: 'tile' | 'cascade') => {
        const { w: aw, h: ah } = area();
        const open = wins.filter((w) => !w.min);
        const cols = Math.ceil(Math.sqrt(open.length));
        const rows = Math.ceil(open.length / Math.max(1, cols));
        setWins((ws) => {
            let i = 0;
            return ws.map((w) => {
                if (w.min) return w;
                const k = i++;
                if (mode === 'cascade') {
                    return {
                        ...w,
                        max: false,
                        x: 40 + k * 36,
                        y: 24 + k * 32,
                        w: Math.round(aw * 0.6),
                        h: Math.round(ah * 0.7),
                    };
                }
                const cw = Math.floor(aw / cols);
                const rh = Math.floor(ah / rows);
                return {
                    ...w,
                    max: false,
                    x: (k % cols) * cw,
                    y: Math.floor(k / cols) * rh,
                    w: cw,
                    h: rh,
                };
            });
        });
        setStartOpen(false);
    };

    // Drag and resize ------------------------------------------------------
    const drag = (e: React.PointerEvent, win: Win, mode: 'move' | string) => {
        if (e.button !== 0) return;
        e.preventDefault();
        focus(win.id);
        const start = {
            px: e.clientX,
            py: e.clientY,
            ...(win.max && mode === 'move'
                ? { ...win, x: e.clientX - win.w / 2, y: 0 }
                : win),
        };
        if (win.max && mode === 'move')
            update(win.id, {
                max: false,
                x: start.x,
                y: start.y,
                ...(win.prev ? { w: win.prev.w, h: win.prev.h } : {}),
            });
        const w0 = win.max && win.prev ? win.prev.w : win.w;
        const h0 = win.max && win.prev ? win.prev.h : win.h;
        setDragging(true);
        let snap: 'left' | 'right' | 'top' | null = null;
        const move = (ev: PointerEvent) => {
            const dx = ev.clientX - start.px;
            const dy = ev.clientY - start.py;
            const { w: aw, h: ah } = area();
            if (mode === 'move') {
                snap =
                    ev.clientX <= 4
                        ? 'left'
                        : ev.clientX >= aw - 4
                          ? 'right'
                          : ev.clientY <= 2
                            ? 'top'
                            : null;
                update(win.id, {
                    x: Math.min(aw - 80, Math.max(-w0 + 120, start.x + dx)),
                    y: Math.min(ah - 40, Math.max(0, start.y + dy)),
                });
                return;
            }
            const r: Rect = { x: start.x, y: start.y, w: w0, h: h0 };
            if (mode.includes('e')) r.w = Math.max(MIN_W, w0 + dx);
            if (mode.includes('s')) r.h = Math.max(MIN_H, h0 + dy);
            if (mode.includes('w')) {
                r.w = Math.max(MIN_W, w0 - dx);
                r.x = start.x + (w0 - r.w);
            }
            update(win.id, { ...r, max: false });
        };
        const up = () => {
            window.removeEventListener('pointermove', move);
            window.removeEventListener('pointerup', up);
            setDragging(false);
            const { w: aw, h: ah } = area();
            if (snap === 'top') toggleMax(win.id);
            else if (snap)
                update(win.id, {
                    x: snap === 'left' ? 0 : aw / 2,
                    y: 0,
                    w: aw / 2,
                    h: ah,
                    max: false,
                });
        };
        window.addEventListener('pointermove', move);
        window.addEventListener('pointerup', up);
    };

    // A click inside a window's page focuses that window.
    useEffect(() => {
        const onBlur = () =>
            setTimeout(() => {
                const el = document.activeElement;
                for (const [id, f] of frames) if (f === el) focus(id);
            });
        window.addEventListener('blur', onBlur);
        return () => window.removeEventListener('blur', onBlur);
    }, [focus]);

    // Shortcuts, from the desktop or forwarded by a window.
    useEffect(() => {
        if (!active) return;
        const handle = (code: string) => {
            const ordered = [...wins].sort((a, b) => a.id.localeCompare(b.id));
            if (/^Digit[1-9]$/.test(code)) {
                const w = wins[Number(code.slice(5)) - 1];
                if (w) focus(w.id);
                return;
            }
            if (!focused) return;
            if (code === 'KeyW') close(focused);
            if (code === 'KeyM') minimize(focused);
            if (code === 'ArrowUp') toggleMax(focused);
            void ordered;
        };
        const onKey = (e: KeyboardEvent) => {
            if (
                e.altKey &&
                !e.ctrlKey &&
                !e.metaKey &&
                /^(Digit[1-9]|KeyW|KeyM|ArrowUp)$/.test(e.code)
            ) {
                e.preventDefault();
                handle(e.code);
            }
        };
        const onMessage = (e: MessageEvent) => {
            if (e.origin !== window.location.origin) return;
            const d = e.data as { type?: string; code?: string };
            if (d?.type === DESKTOP_KEY_MESSAGE && typeof d.code === 'string')
                handle(d.code);
        };
        window.addEventListener('keydown', onKey);
        window.addEventListener('message', onMessage);
        return () => {
            window.removeEventListener('keydown', onKey);
            window.removeEventListener('message', onMessage);
        };
    }, [active, wins, focused, focus, close, minimize, toggleMax]);

    // Follow navigation inside a window: title, icon and address.
    const onFrameLoad = (id: string) => {
        const f = frames.get(id);
        try {
            const doc = f?.contentDocument;
            const loc = f?.contentWindow?.location;
            if (!doc || !loc) return;
            const app = activeNav(loc.pathname);
            update(id, {
                title: doc.title.split(' · ')[0] || (app ? t(app.label) : ''),
                url: loc.pathname + loc.search,
                ...(app ? { app: app.id } : {}),
            });
        } catch {
            // not same-origin: keep what we have
        }
    };

    if (!active) return null;

    const filtered = apps.filter((a) =>
        t(a.label)
            .toLocaleLowerCase()
            .includes(query.trim().toLocaleLowerCase()),
    );
    const time = new Intl.DateTimeFormat(prefs.locale, {
        hour: prefs.timeFormat === '12h' ? 'numeric' : '2-digit',
        minute: '2-digit',
        hour12: prefs.timeFormat === '12h',
        timeZone: prefs.timeZone,
    }).format(now);
    const date = new Intl.DateTimeFormat(prefs.locale, {
        day: 'numeric',
        month: 'short',
        weekday: 'short',
        timeZone: prefs.timeZone,
    }).format(now);

    return (
        <div
            className="fixed inset-0 z-[60] flex flex-col overflow-hidden text-foreground select-none"
            style={{
                background: WALLPAPERS[wallpaper],
                backgroundSize: WALLPAPER_SIZE[wallpaper],
            }}
            onPointerDown={(e) => {
                if (e.target === e.currentTarget) setSelected(null);
            }}
        >
            {/* Desktop icons */}
            <div
                className="absolute inset-0 bottom-[52px] grid content-start justify-start gap-1 p-4"
                style={{
                    gridTemplateRows: 'repeat(auto-fill, 96px)',
                    gridAutoFlow: 'column',
                    gridAutoColumns: '92px',
                }}
                onPointerDown={(e) => {
                    if (e.target === e.currentTarget) setSelected(null);
                }}
            >
                {apps.map((a) => (
                    <button
                        key={a.id}
                        type="button"
                        onClick={() => setSelected(a.id)}
                        onDoubleClick={() => open(a)}
                        onKeyDown={(e) => e.key === 'Enter' && open(a)}
                        className={cn(
                            'flex flex-col items-center gap-1.5 rounded-xl p-2 text-center outline-none hover:bg-background/30 focus-visible:ring-2 focus-visible:ring-ring',
                            selected === a.id &&
                                'bg-primary/20 ring-1 ring-primary/40',
                        )}
                    >
                        <span className="flex size-12 items-center justify-center rounded-2xl bg-card/90 text-primary shadow-lg ring-1 ring-black/5 backdrop-blur">
                            <a.icon className="size-6" />
                        </span>
                        <span className="line-clamp-2 text-xs font-medium [text-shadow:0_1px_2px_var(--background)]">
                            {t(a.label)}
                        </span>
                    </button>
                ))}
            </div>

            {wins.length === 0 && (
                <p className="pointer-events-none absolute inset-x-0 top-1/2 text-center text-sm text-foreground/70">
                    {t('desktop.noWindows')}
                </p>
            )}

            {/* Windows */}
            {wins.map((w) => {
                const app = appOf(w.app);
                const Icon = app?.icon ?? LayoutGrid;
                const isFocused = w.id === focused;
                const rect = w.max
                    ? {
                          x: 0,
                          y: 0,
                          w: window.innerWidth,
                          h: window.innerHeight - TASKBAR,
                      }
                    : w;
                return (
                    <section
                        key={w.id}
                        role="dialog"
                        aria-label={w.title}
                        onPointerDown={() => !isFocused && focus(w.id)}
                        className={cn(
                            'absolute flex flex-col overflow-hidden border bg-background shadow-2xl transition-[box-shadow,opacity]',
                            w.max ? 'rounded-none' : 'rounded-xl',
                            isFocused
                                ? 'ring-1 shadow-black/30 ring-primary/30'
                                : 'opacity-95 shadow-black/15',
                            w.min && 'pointer-events-none invisible',
                        )}
                        style={{
                            left: rect.x,
                            top: rect.y,
                            width: rect.w,
                            height: rect.h,
                            zIndex: w.z,
                        }}
                    >
                        <header
                            onPointerDown={(e) => drag(e, w, 'move')}
                            onDoubleClick={() => toggleMax(w.id)}
                            className={cn(
                                'flex h-10 shrink-0 cursor-default items-center gap-2 border-b px-3',
                                isFocused ? 'bg-muted' : 'bg-muted/50',
                            )}
                        >
                            <Icon className="size-4 text-primary" />
                            <span className="min-w-0 flex-1 truncate text-sm font-medium">
                                {w.title}
                            </span>
                            {[
                                {
                                    label: t('desktop.reload'),
                                    icon: RotateCw,
                                    run: () =>
                                        frames
                                            .get(w.id)
                                            ?.contentWindow?.location.reload(),
                                },
                                {
                                    label: t('desktop.openInPanel'),
                                    icon: ExternalLink,
                                    run: () => {
                                        exit();
                                        router.push(w.url);
                                    },
                                },
                                {
                                    label: t('desktop.minimize'),
                                    icon: Minus,
                                    run: () => minimize(w.id),
                                },
                                {
                                    label: w.max
                                        ? t('desktop.restore')
                                        : t('desktop.maximize'),
                                    icon: w.max ? Copy : Square,
                                    run: () => toggleMax(w.id),
                                },
                                {
                                    label: t('desktop.close'),
                                    icon: X,
                                    run: () => close(w.id),
                                    danger: true,
                                },
                            ].map((b) => (
                                <button
                                    key={b.label}
                                    type="button"
                                    aria-label={b.label}
                                    title={b.label}
                                    onPointerDown={(e) => e.stopPropagation()}
                                    onClick={b.run}
                                    className={cn(
                                        'flex size-7 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-background hover:text-foreground',
                                        'danger' in b &&
                                            b.danger &&
                                            'hover:bg-destructive hover:text-white',
                                    )}
                                >
                                    <b.icon className="size-3.5" />
                                </button>
                            ))}
                        </header>
                        <iframe
                            ref={(el) => {
                                if (el) frames.set(w.id, el);
                            }}
                            src={w.url}
                            title={w.title}
                            onLoad={() => onFrameLoad(w.id)}
                            className={cn(
                                'w-full flex-1 bg-background',
                                dragging && 'pointer-events-none',
                            )}
                        />
                        {!w.max &&
                            ['e', 's', 'w', 'se', 'sw'].map((edge) => (
                                <div
                                    key={edge}
                                    onPointerDown={(e) => drag(e, w, edge)}
                                    className={cn(
                                        'absolute',
                                        edge === 'e' &&
                                            'top-0 right-0 bottom-0 w-1.5 cursor-ew-resize',
                                        edge === 'w' &&
                                            'top-0 bottom-0 left-0 w-1.5 cursor-ew-resize',
                                        edge === 's' &&
                                            'right-0 bottom-0 left-0 h-1.5 cursor-ns-resize',
                                        edge === 'se' &&
                                            'right-0 bottom-0 size-3 cursor-nwse-resize',
                                        edge === 'sw' &&
                                            'bottom-0 left-0 size-3 cursor-nesw-resize',
                                    )}
                                />
                            ))}
                    </section>
                );
            })}

            {/* Taskbar */}
            <footer
                className="absolute inset-x-0 bottom-0 z-[9999] flex items-center gap-2 border-t border-white/10 bg-background/70 px-2 shadow-[0_-8px_30px_rgba(0,0,0,0.12)] backdrop-blur-2xl"
                style={{ height: TASKBAR }}
            >
                <Popover open={startOpen} onOpenChange={setStartOpen}>
                    <PopoverTrigger asChild>
                        <button
                            type="button"
                            aria-label={t('desktop.start')}
                            className="flex h-10 items-center gap-2 rounded-xl px-2 transition-colors hover:bg-accent"
                        >
                            <BrandMark className="size-8" />
                            <span className="hidden text-sm font-semibold xl:inline">
                                {organization}
                            </span>
                        </button>
                    </PopoverTrigger>
                    <PopoverContent
                        side="top"
                        align="start"
                        sideOffset={10}
                        className="z-[10000] w-[26rem] space-y-3 p-3"
                    >
                        <div className="flex items-center gap-2 rounded-lg border px-2">
                            <Search className="size-4 text-muted-foreground" />
                            <input
                                autoFocus
                                value={query}
                                onChange={(e) => setQuery(e.target.value)}
                                onKeyDown={(e) => {
                                    if (e.key === 'Enter' && filtered[0])
                                        open(filtered[0]);
                                }}
                                placeholder={t('desktop.searchApps')}
                                className="h-9 w-full bg-transparent text-sm outline-none"
                            />
                        </div>
                        <p className="text-xs font-medium tracking-wide text-muted-foreground uppercase">
                            {t('desktop.apps')}
                        </p>
                        <div className="grid grid-cols-4 gap-1">
                            {filtered.map((a) => (
                                <button
                                    key={a.id}
                                    type="button"
                                    onClick={() => open(a)}
                                    className="flex flex-col items-center gap-1.5 rounded-xl p-2 text-center hover:bg-accent"
                                >
                                    <span className="flex size-10 items-center justify-center rounded-xl bg-primary/10 text-primary">
                                        <a.icon className="size-5" />
                                    </span>
                                    <span className="line-clamp-2 text-[11px]">
                                        {t(a.label)}
                                    </span>
                                </button>
                            ))}
                        </div>
                        <div className="grid grid-cols-2 gap-1 border-t pt-2">
                            <button
                                type="button"
                                onClick={() => arrange('tile')}
                                className="flex items-center gap-2 rounded-lg px-2 py-1.5 text-sm hover:bg-accent"
                            >
                                <PanelsTopLeft className="size-4" />{' '}
                                {t('desktop.tile')}
                            </button>
                            <button
                                type="button"
                                onClick={() => arrange('cascade')}
                                className="flex items-center gap-2 rounded-lg px-2 py-1.5 text-sm hover:bg-accent"
                            >
                                <Layers className="size-4" />{' '}
                                {t('desktop.cascade')}
                            </button>
                            <button
                                type="button"
                                onClick={() => {
                                    setWins([]);
                                    setStartOpen(false);
                                }}
                                className="flex items-center gap-2 rounded-lg px-2 py-1.5 text-sm hover:bg-accent"
                            >
                                <X className="size-4" /> {t('desktop.closeAll')}
                            </button>
                            <button
                                type="button"
                                onClick={exit}
                                className="flex items-center gap-2 rounded-lg px-2 py-1.5 text-sm text-destructive hover:bg-destructive/10"
                            >
                                <LogOut className="size-4" />{' '}
                                {t('desktop.exit')}
                            </button>
                        </div>
                        <p className="text-[11px] text-muted-foreground">
                            {t('desktop.keyboard')}
                        </p>
                    </PopoverContent>
                </Popover>

                <span className="h-6 w-px bg-border" />

                <nav
                    aria-label={t('desktop.openWindows')}
                    className="flex min-w-0 flex-1 items-center gap-1 overflow-x-auto"
                >
                    {wins.map((w, i) => {
                        const Icon = appOf(w.app)?.icon ?? LayoutGrid;
                        const on = w.id === focused;
                        return (
                            <button
                                key={w.id}
                                type="button"
                                title={`${w.title} (Alt+${i + 1})`}
                                onClick={() =>
                                    on ? minimize(w.id) : focus(w.id)
                                }
                                className={cn(
                                    'relative flex h-10 max-w-48 shrink-0 items-center gap-2 rounded-xl px-3 text-sm transition-colors hover:bg-accent',
                                    on && 'bg-accent',
                                    w.min && 'opacity-60',
                                )}
                            >
                                <Icon className="size-4 shrink-0 text-primary" />
                                <span className="truncate">{w.title}</span>
                                <span
                                    className={cn(
                                        'absolute bottom-1 left-1/2 h-0.5 -translate-x-1/2 rounded-full bg-primary transition-all',
                                        on ? 'w-6' : 'w-2 opacity-60',
                                    )}
                                />
                            </button>
                        );
                    })}
                </nav>

                <Tooltip>
                    <TooltipTrigger asChild>
                        <span
                            className={cn(
                                'size-2.5 rounded-full',
                                realtime === 'connected'
                                    ? 'bg-success'
                                    : realtime === 'connecting'
                                      ? 'bg-warning'
                                      : 'bg-muted-foreground',
                            )}
                        />
                    </TooltipTrigger>
                    <TooltipContent className="z-[10000]">
                        {t(
                            realtime === 'connected'
                                ? 'topbar.live'
                                : realtime === 'connecting'
                                  ? 'topbar.connecting'
                                  : 'topbar.offline',
                        )}
                    </TooltipContent>
                </Tooltip>
                <button
                    type="button"
                    aria-label={t('desktop.showDesktop')}
                    title={t('desktop.showDesktop')}
                    onClick={() =>
                        setWins((ws) => ws.map((w) => ({ ...w, min: true })))
                    }
                    className="flex h-10 items-center rounded-xl px-2 text-muted-foreground hover:bg-accent"
                >
                    <Maximize2 className="size-4" />
                </button>
                <div className="px-2 text-right leading-tight tabular-nums">
                    <div className="text-sm font-medium">{time}</div>
                    <div className="text-[11px] text-muted-foreground capitalize">
                        {date}
                    </div>
                </div>
            </footer>
        </div>
    );
}
