'use client';

import { Fragment, useEffect, useRef, useState } from 'react';
import { useRouter } from 'next/navigation';
import { Keyboard } from 'lucide-react';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogHeader,
    DialogTitle,
} from '@/components/ui/dialog';
import { Kbd } from '@/components/ui/kbd';
import { useT } from '@/i18n/client';
import { visibleNav } from '@/lib/navigation';

/** True while the user types into a field (shortcuts must not fire). */
function typing(e: KeyboardEvent) {
    const el = e.target as HTMLElement | null;
    if (!el) return false;
    return (
        el.isContentEditable ||
        /^(INPUT|TEXTAREA|SELECT)$/.test(el.tagName) ||
        el.getAttribute('role') === 'combobox' ||
        el.closest('[role="dialog"], [role="menu"], [role="listbox"]') !== null
    );
}

function visible(el: HTMLElement) {
    return (
        el.offsetParent !== null &&
        !el.closest('[inert], [aria-hidden="true"]') &&
        !(el as HTMLButtonElement).disabled &&
        el.getAttribute('aria-disabled') !== 'true'
    );
}

type PageShortcut = { key: string; label: string };

/** Shortcuts declared by the page: elements with data-shortcut. */
function pageShortcuts(): PageShortcut[] {
    const seen = new Set<string>();
    const out: PageShortcut[] = [];
    document.querySelectorAll<HTMLElement>('[data-shortcut]').forEach((el) => {
        const key = el.dataset.shortcut!.toLowerCase();
        if (seen.has(key) || !visible(el)) return;
        seen.add(key);
        const label = (el.getAttribute('aria-label') ?? el.textContent ?? '')
            .replace(new RegExp(`\\s*${key}\\s*$`, 'i'), '')
            .trim();
        out.push({ key, label });
    });
    return out;
}

/**
 * Keyboard shortcuts for the whole panel:
 *  - a single key clicks the visible element with that data-shortcut
 *    (Button's `shortcut` prop), e.g. N for "new" on every list page;
 *  - G then a letter opens a section;
 *  - / focuses the page's search field; ? lists everything.
 * Nothing fires while typing or with Ctrl/Cmd/Alt held (except ⌘K, handled
 * by the top bar).
 */
export function Shortcuts({
    permissions,
    restricted,
}: {
    permissions: string[];
    restricted: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const [help, setHelp] = useState(false);
    const [page, setPage] = useState<PageShortcut[]>([]);
    const pendingG = useRef<number>(0);
    const nav = visibleNav(permissions, restricted).filter((n) => n.goKey);

    useEffect(() => {
        const onKey = (e: KeyboardEvent) => {
            if (e.ctrlKey || e.metaKey || e.altKey || e.defaultPrevented)
                return;
            if (typing(e)) return;
            const key = e.key.toLowerCase();

            if (e.key === '?') {
                e.preventDefault();
                setPage(pageShortcuts());
                setHelp(true);
                return;
            }
            if (Date.now() - pendingG.current < 1200) {
                pendingG.current = 0;
                const item = nav.find((n) => n.goKey === key);
                if (item) {
                    e.preventDefault();
                    router.push(item.href);
                }
                return;
            }
            if (key === 'g' && !restricted) {
                pendingG.current = Date.now();
                return;
            }
            if (e.key === '/') {
                const search = Array.from(
                    document.querySelectorAll<HTMLInputElement>(
                        'main input[type="search"], main [data-search], main input[name="q"]',
                    ),
                ).find(visible);
                if (search) {
                    e.preventDefault();
                    search.focus();
                    search.select?.();
                }
                return;
            }
            if (e.shiftKey) return;
            const target = Array.from(
                document.querySelectorAll<HTMLElement>(
                    `[data-shortcut="${CSS.escape(key)}" i]`,
                ),
            ).find(visible);
            if (target) {
                e.preventDefault();
                target.focus({ preventScroll: true });
                target.click();
            }
        };
        const openHelp = () => {
            setPage(pageShortcuts());
            setHelp(true);
        };
        window.addEventListener('keydown', onKey);
        window.addEventListener('messora:shortcuts-open', openHelp);
        return () => {
            window.removeEventListener('keydown', onKey);
            window.removeEventListener('messora:shortcuts-open', openHelp);
        };
    }, [nav, restricted, router]);

    const row = (label: string, keys: React.ReactNode) => (
        <li className="flex items-center justify-between gap-4 py-1.5 text-sm">
            <span className="truncate">{label}</span>
            <span className="flex shrink-0 items-center gap-1 text-muted-foreground">
                {keys}
            </span>
        </li>
    );

    return (
        <Dialog open={help} onOpenChange={setHelp}>
            <DialogContent className="sm:max-w-2xl">
                <DialogHeader>
                    <DialogTitle className="flex items-center gap-2">
                        <Keyboard className="size-5 text-primary" />
                        {t('shortcuts.title')}
                    </DialogTitle>
                    <DialogDescription>
                        {t('shortcuts.description')}
                    </DialogDescription>
                </DialogHeader>
                <div className="grid gap-6 sm:grid-cols-2">
                    <div className="space-y-5">
                        <section>
                            <h3 className="mb-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">
                                {t('shortcuts.thisPage')}
                            </h3>
                            {page.length === 0 ? (
                                <p className="py-1.5 text-sm text-muted-foreground">
                                    {t('shortcuts.none')}
                                </p>
                            ) : (
                                <ul className="divide-y">
                                    {page.map((s) => (
                                        <Fragment key={s.key}>
                                            {row(s.label, <Kbd>{s.key}</Kbd>)}
                                        </Fragment>
                                    ))}
                                </ul>
                            )}
                        </section>
                        <section>
                            <h3 className="mb-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">
                                {t('shortcuts.general')}
                            </h3>
                            <ul className="divide-y">
                                {row(
                                    t('shortcuts.palette'),
                                    <>
                                        <Kbd>⌘</Kbd>
                                        <Kbd>K</Kbd>
                                    </>,
                                )}
                                {row(t('shortcuts.focusSearch'), <Kbd>/</Kbd>)}
                                {row(t('shortcuts.help'), <Kbd>?</Kbd>)}
                            </ul>
                        </section>
                    </div>
                    <section>
                        <h3 className="mb-1 text-xs font-medium tracking-wide text-muted-foreground uppercase">
                            {t('shortcuts.navigation')}
                        </h3>
                        <ul className="divide-y">
                            {nav.map((n) => (
                                <Fragment key={n.id}>
                                    {row(
                                        t(n.label),
                                        <>
                                            <Kbd>G</Kbd>
                                            <span className="text-[10px]">
                                                {t('shortcuts.then')}
                                            </span>
                                            <Kbd>{n.goKey}</Kbd>
                                        </>,
                                    )}
                                </Fragment>
                            ))}
                        </ul>
                    </section>
                </div>
            </DialogContent>
        </Dialog>
    );
}
