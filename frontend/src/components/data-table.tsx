'use client';

import {
    useCallback,
    useEffect,
    useMemo,
    useRef,
    useState,
    useTransition,
} from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import {
    ArrowDown,
    ArrowUp,
    ArrowUpDown,
    ChevronLeft,
    ChevronRight,
    ChevronsLeft,
    ChevronsRight,
    Columns3,
    Loader2,
    Rows3,
    Rows4,
    X,
} from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Checkbox } from '@/components/ui/checkbox';
import {
    DropdownMenu,
    DropdownMenuCheckboxItem,
    DropdownMenuContent,
    DropdownMenuLabel,
    DropdownMenuSeparator,
    DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Segmented } from '@/components/ui/segmented';
import { useLocale, useT } from '@/i18n/client';
import { intlLocale } from '@/i18n/config';
import { cn } from '@/lib/utils';
import { useContextMenu, type MenuGroup } from '@/components/context-menu';
import { CheckSquare, Square as SquareIcon } from 'lucide-react';

export type Column<T> = {
    id: string;
    header: React.ReactNode;
    cell: (row: T) => React.ReactNode;
    /** Value of the "sort" URL parameter for ascending / descending order. */
    sort?: { asc: string; desc: string };
    className?: string;
    /** May be hidden from the column menu (default true). */
    hideable?: boolean;
    defaultHidden?: boolean;
};

/**
 * What is selected: explicit ids, or every item matching the current
 * filter except some.
 */
export type Selection =
    | { mode: 'ids'; ids: string[] }
    | { mode: 'all'; exclude: string[]; total: number };

export function selectionCount(s: Selection) {
    return s.mode === 'ids' ? s.ids.length : s.total - s.exclude.length;
}

const PAGE_SIZES = [25, 50, 100, 200];

/**
 * A list with selection (shift-click ranges, whole page, everything that
 * matches), sorting, column visibility, density and numbered pages. Page,
 * size and sort live in the URL so the server renders the right page.
 */
export function DataTable<T>({
    rows,
    columns,
    rowId,
    total,
    page,
    pageSize,
    sort,
    selectable = true,
    bulkActions,
    rowActions,
    onRowClick,
    empty,
    storageKey,
    rowTestId,
    rowMenu,
    onNavigate,
    toolbar,
}: {
    rows: T[];
    columns: Column<T>[];
    rowId: (row: T) => string;
    total: number;
    page: number;
    pageSize: number;
    sort?: string;
    selectable?: boolean;
    /** Rendered in the floating bar while something is selected. */
    bulkActions?: (selection: Selection, clear: () => void) => React.ReactNode;
    rowActions?: (row: T) => React.ReactNode;
    onRowClick?: (row: T) => void;
    empty?: React.ReactNode;
    /** Remembers hidden columns and density per browser. */
    storageKey: string;
    rowTestId?: string;
    /** Extra right-click items for a row. */
    rowMenu?: (row: T) => MenuGroup[];
    /**
     * Handles page, size and sort changes instead of the URL (lists that are
     * sorted and paged in the browser, see ClientDataTable).
     */
    onNavigate?: (patch: Record<string, string | number | null>) => void;
    /** Shown at the start of the top bar (search, filters). */
    toolbar?: React.ReactNode;
}) {
    const t = useT();
    // The language is fixed per request, so server and browser agree.
    const numberLocale = intlLocale(useLocale());
    const n = (v: number) => v.toLocaleString(numberLocale);
    const router = useRouter();
    const pathname = usePathname();
    const params = useSearchParams();
    const [navigating, startNav] = useTransition();
    const [ids, setIds] = useState<Set<string>>(new Set());
    const [all, setAll] = useState<{ exclude: Set<string> } | null>(null);
    const [lastClicked, setLastClicked] = useState<number | null>(null);
    const [prefs, setPrefs] = useState<{
        hidden: string[];
        density: 'compact' | 'comfortable';
    }>(() => ({
        hidden: columns.filter((c) => c.defaultHidden).map((c) => c.id),
        density: 'comfortable',
    }));

    // Stored preferences load after hydration (the server cannot know them).
    useEffect(() => {
        try {
            const raw = localStorage.getItem(`table:${storageKey}`);
            // eslint-disable-next-line react-hooks/set-state-in-effect -- one-time restore from the browser
            if (raw) setPrefs((p) => ({ ...p, ...JSON.parse(raw) }));
        } catch {
            // storage unavailable: keep defaults
        }
    }, [storageKey]);
    const savePrefs = (next: typeof prefs) => {
        setPrefs(next);
        try {
            localStorage.setItem(`table:${storageKey}`, JSON.stringify(next));
        } catch {
            // ignore
        }
    };

    const clear = useCallback(() => {
        setIds(new Set());
        setAll(null);
        setLastClicked(null);
    }, []);

    // A new page, filter or sort starts a new selection of ids (a select-all
    // stays meaningful only for the same filter, which changes the URL).
    const urlKey = params.toString();
    const prevKey = useRef(urlKey);
    useEffect(() => {
        if (prevKey.current === urlKey) return;
        const onlyPage = (k: string) => {
            const p = new URLSearchParams(k);
            p.delete('page');
            p.delete('limit');
            return p.toString();
        };
        if (onlyPage(prevKey.current) !== onlyPage(urlKey)) clear();
        prevKey.current = urlKey;
    }, [urlKey, clear]);

    useEffect(() => {
        const onKey = (e: KeyboardEvent) => {
            if (e.key === 'Escape' && (ids.size > 0 || all)) clear();
        };
        window.addEventListener('keydown', onKey);
        return () => window.removeEventListener('keydown', onKey);
    }, [ids, all, clear]);

    const visible = columns.filter((c) => !prefs.hidden.includes(c.id));
    const pageIds = rows.map(rowId);
    const isSelected = (id: string) =>
        all ? !all.exclude.has(id) : ids.has(id);
    const selectedOnPage = pageIds.filter(isSelected).length;
    const allOnPage = pageIds.length > 0 && selectedOnPage === pageIds.length;
    const selection: Selection = all
        ? { mode: 'all', exclude: [...all.exclude], total }
        : { mode: 'ids', ids: [...ids] };
    const count = selectionCount(selection);

    const toggle = (index: number, shift: boolean) => {
        const id = pageIds[index]!;
        const on = !isSelected(id);
        const range =
            shift && lastClicked !== null
                ? pageIds.slice(
                      Math.min(lastClicked, index),
                      Math.max(lastClicked, index) + 1,
                  )
                : [id];
        setLastClicked(index);
        if (all) {
            const ex = new Set(all.exclude);
            for (const r of range) {
                if (on) ex.delete(r);
                else ex.add(r);
            }
            setAll({ exclude: ex });
        } else {
            const next = new Set(ids);
            for (const r of range) {
                if (on) next.add(r);
                else next.delete(r);
            }
            setIds(next);
        }
    };
    const togglePage = () => {
        if (all) {
            const ex = new Set(all.exclude);
            for (const id of pageIds) {
                if (allOnPage) ex.add(id);
                else ex.delete(id);
            }
            setAll({ exclude: ex });
            return;
        }
        const next = new Set(ids);
        for (const id of pageIds) {
            if (allOnPage) next.delete(id);
            else next.add(id);
        }
        setIds(next);
    };

    const go = (patch: Record<string, string | number | null>) =>
        onNavigate
            ? onNavigate(patch)
            : startNav(() => {
                  const p = new URLSearchParams(params.toString());
                  for (const [k, v] of Object.entries(patch)) {
                      if (v === null || v === '') p.delete(k);
                      else p.set(k, String(v));
                  }
                  router.push(`${pathname}?${p}`, { scroll: false });
              });

    const pages = Math.max(1, Math.ceil(total / pageSize));
    const from = total === 0 ? 0 : (page - 1) * pageSize + 1;
    const to = Math.min(total, page * pageSize);
    const numbers = useMemo(() => {
        const set = new Set(
            [1, pages, page - 1, page, page + 1].filter(
                (x) => x >= 1 && x <= pages,
            ),
        );
        const sorted = [...set].sort((a, b) => a - b);
        const out: (number | '…')[] = [];
        sorted.forEach((x, i) => {
            if (i > 0 && x - sorted[i - 1]! > 1) out.push('…');
            out.push(x);
        });
        return out;
    }, [page, pages]);

    const bodyMenu = useContextMenu((target) => {
        const tr = target.closest<HTMLElement>('tr[data-row-index]');
        if (!tr) return null;
        const i = Number(tr.dataset.rowIndex);
        const row = rows[i];
        if (!row) return null;
        const groups: MenuGroup[] = [...(rowMenu?.(row) ?? [])];
        if (selectable) {
            const on = isSelected(pageIds[i]!);
            groups.push([
                {
                    id: 'select',
                    label: on
                        ? t('contextMenu.deselect')
                        : t('contextMenu.select'),
                    icon: on ? SquareIcon : CheckSquare,
                    run: () => toggle(i, false),
                },
            ]);
        }
        return groups;
    });

    const cellPad = prefs.density === 'compact' ? 'px-3 py-1.5' : 'px-3 py-3';

    return (
        <div className="space-y-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
                {toolbar && (
                    <div className="flex min-w-0 flex-1 flex-wrap items-center gap-2">
                        {toolbar}
                    </div>
                )}
                <p className="text-sm text-muted-foreground tabular-nums">
                    {t('table.rows', {
                        from: n(from),
                        to: n(to),
                        total: n(total),
                    })}
                    {navigating && (
                        <Loader2 className="ml-2 inline size-3.5 animate-spin" />
                    )}
                </p>
                <div className="flex items-center gap-2">
                    <Segmented
                        aria-label={t('table.density')}
                        value={prefs.density}
                        onValueChange={(density) =>
                            savePrefs({ ...prefs, density })
                        }
                        options={[
                            {
                                value: 'comfortable',
                                label: (
                                    <Rows3
                                        className="size-4"
                                        aria-label={t('table.comfortable')}
                                    />
                                ),
                            },
                            {
                                value: 'compact',
                                label: (
                                    <Rows4
                                        className="size-4"
                                        aria-label={t('table.compact')}
                                    />
                                ),
                            },
                        ]}
                    />
                    <DropdownMenu>
                        <DropdownMenuTrigger asChild>
                            <Button variant="outline" size="sm">
                                <Columns3 />
                                {t('table.columns')}
                            </Button>
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="end" className="w-52">
                            <DropdownMenuLabel>
                                {t('table.columns')}
                            </DropdownMenuLabel>
                            <DropdownMenuSeparator />
                            {columns
                                .filter((c) => c.hideable !== false)
                                .map((c) => (
                                    <DropdownMenuCheckboxItem
                                        key={c.id}
                                        checked={!prefs.hidden.includes(c.id)}
                                        onSelect={(e) => e.preventDefault()}
                                        onCheckedChange={(on) =>
                                            savePrefs({
                                                ...prefs,
                                                hidden: on
                                                    ? prefs.hidden.filter(
                                                          (h) => h !== c.id,
                                                      )
                                                    : [...prefs.hidden, c.id],
                                            })
                                        }
                                    >
                                        {c.header}
                                    </DropdownMenuCheckboxItem>
                                ))}
                        </DropdownMenuContent>
                    </DropdownMenu>
                </div>
            </div>

            {selectable && (allOnPage || all) && total > pageIds.length && (
                <div className="flex flex-wrap items-center justify-center gap-2 rounded-xl border border-primary/20 bg-primary/5 px-4 py-2 text-sm">
                    {all ? (
                        <>
                            {t('table.allSelected', { count: n(total) })}
                            <button
                                type="button"
                                className="font-medium text-primary hover:underline"
                                onClick={clear}
                            >
                                {t('table.clearSelection')}
                            </button>
                        </>
                    ) : (
                        <>
                            {t('table.pageSelected', {
                                count: n(pageIds.length),
                            })}
                            <button
                                type="button"
                                className="font-medium text-primary hover:underline"
                                onClick={() => setAll({ exclude: new Set() })}
                            >
                                {t('table.selectAllMatching', {
                                    count: n(total),
                                })}
                            </button>
                        </>
                    )}
                </div>
            )}

            <div className="overflow-hidden rounded-2xl border bg-card shadow-xs">
                <div className="max-h-[calc(100svh-18rem)] overflow-auto">
                    <table
                        className={cn(
                            'w-full caption-bottom text-sm',
                            navigating && 'opacity-60 transition-opacity',
                        )}
                    >
                        <thead className="sticky top-0 z-10 bg-muted/80 backdrop-blur">
                            <tr className="border-b">
                                {selectable && (
                                    <th className="w-10 px-3">
                                        <Checkbox
                                            aria-label={t('table.selectAll')}
                                            checked={
                                                allOnPage
                                                    ? true
                                                    : selectedOnPage > 0
                                                      ? 'indeterminate'
                                                      : false
                                            }
                                            onCheckedChange={togglePage}
                                        />
                                    </th>
                                )}
                                {visible.map((c) => {
                                    const dir = c.sort
                                        ? sort === c.sort.asc
                                            ? 'asc'
                                            : sort === c.sort.desc
                                              ? 'desc'
                                              : null
                                        : null;
                                    return (
                                        <th
                                            key={c.id}
                                            aria-sort={
                                                dir === 'asc'
                                                    ? 'ascending'
                                                    : dir === 'desc'
                                                      ? 'descending'
                                                      : undefined
                                            }
                                            className={cn(
                                                'h-10 px-3 text-left align-middle text-xs font-medium tracking-wide whitespace-nowrap text-muted-foreground uppercase',
                                                c.className,
                                            )}
                                        >
                                            {c.sort ? (
                                                <button
                                                    type="button"
                                                    onClick={() =>
                                                        go({
                                                            sort:
                                                                dir === 'asc'
                                                                    ? c.sort!
                                                                          .desc
                                                                    : c.sort!
                                                                          .asc,
                                                            page: null,
                                                        })
                                                    }
                                                    className="-mx-1 inline-flex items-center gap-1 rounded px-1 py-0.5 uppercase hover:bg-accent hover:text-foreground"
                                                    title={
                                                        dir === 'asc'
                                                            ? t(
                                                                  'table.sortDesc',
                                                              )
                                                            : t('table.sortAsc')
                                                    }
                                                >
                                                    {c.header}
                                                    {dir === 'asc' ? (
                                                        <ArrowUp className="size-3.5 text-primary" />
                                                    ) : dir === 'desc' ? (
                                                        <ArrowDown className="size-3.5 text-primary" />
                                                    ) : (
                                                        <ArrowUpDown className="size-3.5 opacity-40" />
                                                    )}
                                                </button>
                                            ) : (
                                                c.header
                                            )}
                                        </th>
                                    );
                                })}
                                {rowActions && (
                                    <th className="w-12 px-3">
                                        <span className="sr-only">
                                            {t('table.actions')}
                                        </span>
                                    </th>
                                )}
                            </tr>
                        </thead>
                        <tbody {...bodyMenu}>
                            {rows.length === 0 && (
                                <tr>
                                    <td
                                        colSpan={visible.length + 2}
                                        className="p-0"
                                    >
                                        {empty}
                                    </td>
                                </tr>
                            )}
                            {rows.map((row, i) => {
                                const id = pageIds[i]!;
                                const on = isSelected(id);
                                return (
                                    <tr
                                        key={id}
                                        data-state={on ? 'selected' : undefined}
                                        data-testid={rowTestId}
                                        data-row-index={i}
                                        onClick={
                                            onRowClick
                                                ? (e) => {
                                                      if (
                                                          (
                                                              e.target as HTMLElement
                                                          ).closest(
                                                              'a,button,[role="checkbox"],[role="menuitem"]',
                                                          )
                                                      )
                                                          return;
                                                      onRowClick(row);
                                                  }
                                                : undefined
                                        }
                                        className={cn(
                                            'group border-b transition-colors last:border-0 hover:bg-muted/40',
                                            on &&
                                                'bg-primary/5 hover:bg-primary/10',
                                            onRowClick && 'cursor-pointer',
                                        )}
                                    >
                                        {selectable && (
                                            <td className={cn('w-10', cellPad)}>
                                                <Checkbox
                                                    aria-label={t(
                                                        'table.selectRow',
                                                    )}
                                                    checked={on}
                                                    onClick={(e) => {
                                                        e.preventDefault();
                                                        toggle(i, e.shiftKey);
                                                    }}
                                                />
                                            </td>
                                        )}
                                        {visible.map((c) => (
                                            <td
                                                key={c.id}
                                                className={cn(
                                                    'align-middle',
                                                    cellPad,
                                                    c.className,
                                                )}
                                            >
                                                {c.cell(row)}
                                            </td>
                                        ))}
                                        {rowActions && (
                                            <td
                                                className={cn(
                                                    'text-right',
                                                    cellPad,
                                                )}
                                            >
                                                {rowActions(row)}
                                            </td>
                                        )}
                                    </tr>
                                );
                            })}
                        </tbody>
                    </table>
                </div>
                <div className="flex flex-wrap items-center justify-between gap-3 border-t bg-muted/20 px-3 py-2">
                    <div className="flex items-center gap-2 text-sm text-muted-foreground">
                        {t('table.rowsPerPage')}
                        <Select
                            value={String(pageSize)}
                            onValueChange={(v) => go({ limit: v, page: null })}
                        >
                            <SelectTrigger size="sm" className="w-20">
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {PAGE_SIZES.map((s) => (
                                    <SelectItem key={s} value={String(s)}>
                                        {s}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    </div>
                    <nav
                        className="flex items-center gap-1"
                        aria-label={t('table.page')}
                    >
                        <Button
                            variant="ghost"
                            size="icon-sm"
                            disabled={page <= 1}
                            onClick={() => go({ page: null })}
                            aria-label={t('table.first')}
                        >
                            <ChevronsLeft />
                        </Button>
                        <Button
                            variant="ghost"
                            size="icon-sm"
                            disabled={page <= 1}
                            onClick={() =>
                                go({ page: page - 1 > 1 ? page - 1 : null })
                            }
                            aria-label={t('table.previous')}
                        >
                            <ChevronLeft />
                        </Button>
                        {numbers.map((x, i) =>
                            x === '…' ? (
                                <span
                                    key={`e${i}`}
                                    className="px-1 text-muted-foreground"
                                >
                                    …
                                </span>
                            ) : (
                                <Button
                                    key={x}
                                    size="icon-sm"
                                    variant={x === page ? 'default' : 'ghost'}
                                    aria-current={
                                        x === page ? 'page' : undefined
                                    }
                                    className="min-w-7 tabular-nums"
                                    onClick={() =>
                                        go({ page: x > 1 ? x : null })
                                    }
                                >
                                    {x}
                                </Button>
                            ),
                        )}
                        <Button
                            variant="ghost"
                            size="icon-sm"
                            disabled={page >= pages}
                            onClick={() => go({ page: page + 1 })}
                            aria-label={t('table.next')}
                        >
                            <ChevronRight />
                        </Button>
                        <Button
                            variant="ghost"
                            size="icon-sm"
                            disabled={page >= pages}
                            onClick={() => go({ page: pages })}
                            aria-label={t('table.last')}
                        >
                            <ChevronsRight />
                        </Button>
                    </nav>
                </div>
            </div>

            {selectable && count > 0 && bulkActions && (
                <div className="pointer-events-none fixed inset-x-0 bottom-6 z-40 flex justify-center px-4">
                    <div className="pointer-events-auto flex max-w-full animate-in flex-wrap items-center gap-2 rounded-2xl border bg-popover/95 p-2 pl-4 text-popover-foreground shadow-2xl shadow-black/15 backdrop-blur-xl fade-in-0 slide-in-from-bottom-4">
                        <span className="text-sm font-medium tabular-nums">
                            {all
                                ? t('table.selectedAll', { count: n(count) })
                                : t('table.selected', { count: n(count) })}
                        </span>
                        <span className="mx-1 h-5 w-px bg-border" />
                        {bulkActions(selection, clear)}
                        <Button
                            variant="ghost"
                            size="icon-sm"
                            onClick={clear}
                            aria-label={t('table.clearSelection')}
                        >
                            <X />
                        </Button>
                    </div>
                </div>
            )}
        </div>
    );
}
