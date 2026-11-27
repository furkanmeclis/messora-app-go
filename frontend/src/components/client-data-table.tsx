'use client';

import { useEffect, useMemo, useState } from 'react';
import { Info, Search } from 'lucide-react';
import {
    DataTable,
    type Column,
    type Selection,
} from '@/components/data-table';
import type { MenuGroup } from '@/components/context-menu';
import { Input } from '@/components/ui/input';
import type { TablePaging } from '@/hooks/use-cursor-rows';
import { useLocale, useT } from '@/i18n/client';
import { intlLocale } from '@/i18n/config';
import { MAX_LOADED_ROWS, rowsWanted } from '@/lib/paging';

export type ClientColumn<T> = Omit<Column<T>, 'sort'> & {
    /** Makes the column sortable by this value. */
    sortValue?: (row: T) => string | number | Date | null | undefined;
};

/**
 * DataTable for lists the page holds in the browser: sorting, paging and
 * search happen there; selection, bulk actions, column visibility, density
 * and the row menu work as in server-paged tables.
 *
 * With `paging` (a cursor-paged API, see useCursorRows) the rows arrive in
 * chunks and the table's own pager is the only control: it keeps the next
 * page loaded, goes on past the last loaded page while more rows exist, and
 * loads everything (up to a cap) for the last page, a search or a sort.
 */
export function ClientDataTable<T>({
    rows,
    columns,
    rowId,
    storageKey,
    searchText,
    searchPlaceholder,
    toolbar,
    initialSort,
    pageSize: initialPageSize = 25,
    selectable = false,
    bulkActions,
    rowActions,
    rowMenu,
    onRowClick,
    empty,
    noMatch,
    rowTestId,
    paging,
}: {
    rows: T[];
    columns: ClientColumn<T>[];
    rowId: (row: T) => string;
    storageKey: string;
    /** Text a search matches against; no search box without it. */
    searchText?: (row: T) => string;
    searchPlaceholder?: string;
    /** Extra filters beside the search box. */
    toolbar?: React.ReactNode;
    /** "<column id>" ascending or "-<column id>" descending. */
    initialSort?: string;
    pageSize?: number;
    selectable?: boolean;
    /** The selected rows' ids (a select-all is resolved to ids). */
    bulkActions?: (ids: string[], clear: () => void) => React.ReactNode;
    rowActions?: (row: T) => React.ReactNode;
    rowMenu?: (row: T) => MenuGroup[];
    onRowClick?: (row: T) => void;
    empty?: React.ReactNode;
    /** Shown when the search matches nothing (rows exist). */
    noMatch?: React.ReactNode;
    rowTestId?: string;
    /** Rows beyond `rows` can be loaded (see useCursorRows). */
    paging?: TablePaging;
}) {
    const t = useT();
    const locale = intlLocale(useLocale());
    const [q, setQ] = useState('');
    const [sort, setSort] = useState(initialSort ?? '');
    // Infinity: the last page, however many rows arrive.
    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(initialPageSize);

    // A search or another order is only right over every row: load them
    // (up to the cap); otherwise keep one page ahead of the one shown.
    const searching = q.trim() !== '';
    const wantAll = searching || sort !== (initialSort ?? '');
    const want = rowsWanted({ page, pageSize, wantAll });
    const hasMore = paging?.hasMore ?? false;
    const ensure = paging?.ensure;
    useEffect(() => {
        if (ensure && hasMore && rows.length < want) ensure(want);
    }, [ensure, hasMore, rows.length, want]);

    const filtered = useMemo(() => {
        const needle = q.trim().toLocaleLowerCase(locale);
        let out =
            needle && searchText
                ? rows.filter((r) =>
                      searchText(r).toLocaleLowerCase(locale).includes(needle),
                  )
                : rows;
        const desc = sort.startsWith('-');
        const col = columns.find((c) => c.id === (desc ? sort.slice(1) : sort));
        if (col?.sortValue) {
            const val = col.sortValue;
            const collator = new Intl.Collator(locale, {
                numeric: true,
                sensitivity: 'base',
            });
            out = [...out].sort((a, b) => {
                const x = val(a);
                const y = val(b);
                if (x == null && y == null) return 0;
                if (x == null) return 1; // empty values last either way
                if (y == null) return -1;
                const c =
                    typeof x === 'string' && typeof y === 'string'
                        ? collator.compare(x, y)
                        : Number(x) - Number(y);
                return desc ? -c : c;
            });
        }
        return out;
    }, [rows, q, sort, columns, searchText, locale]);

    const pages = Math.max(1, Math.ceil(filtered.length / pageSize));
    const current = Math.min(page, pages);
    const shown = filtered.slice((current - 1) * pageSize, current * pageSize);

    const resolve = (s: Selection) => {
        if (s.mode === 'ids') return s.ids;
        const ex = new Set(s.exclude);
        return filtered.map(rowId).filter((id) => !ex.has(id));
    };

    return (
        <DataTable
            rows={shown}
            columns={columns.map(({ sortValue, ...c }) => ({
                ...c,
                sort: sortValue ? { asc: c.id, desc: `-${c.id}` } : undefined,
            }))}
            rowId={rowId}
            total={filtered.length}
            page={current}
            pageSize={pageSize}
            sort={sort}
            selectable={selectable}
            bulkActions={
                bulkActions
                    ? (selection, clear) =>
                          bulkActions(resolve(selection), clear)
                    : undefined
            }
            rowActions={rowActions}
            rowMenu={rowMenu}
            onRowClick={onRowClick}
            empty={q && rows.length > 0 ? (noMatch ?? empty) : empty}
            storageKey={storageKey}
            rowTestId={rowTestId}
            more={hasMore}
            loading={paging?.loading}
            onNavigate={(patch) => {
                if ('sort' in patch) setSort(String(patch.sort ?? ''));
                if ('limit' in patch && patch.limit)
                    setPageSize(Number(patch.limit));
                if ('page' in patch)
                    setPage(
                        patch.page === 'last'
                            ? Infinity
                            : patch.page
                              ? Number(patch.page)
                              : 1,
                    );
            }}
            toolbar={
                searchText || toolbar || paging?.capped ? (
                    <>
                        {searchText && (
                            <div className="relative w-full sm:w-64">
                                <Search className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-muted-foreground" />
                                <Input
                                    type="search"
                                    aria-label={searchPlaceholder}
                                    placeholder={searchPlaceholder}
                                    className="pl-8"
                                    value={q}
                                    onChange={(e) => {
                                        setQ(e.target.value);
                                        setPage(1);
                                    }}
                                />
                            </div>
                        )}
                        {hasMore && wantAll && (
                            <span
                                className="inline-flex items-center gap-1 text-xs text-muted-foreground"
                                title={t('table.loadedOnlyHint')}
                                data-testid="table-loaded-only"
                            >
                                <Info className="size-3.5" />
                                {t('table.loadedOnly')}
                            </span>
                        )}
                        {toolbar}
                        {paging?.capped && (
                            <p
                                className="w-full text-xs text-muted-foreground"
                                data-testid="table-capped"
                            >
                                {t('table.capped', {
                                    count: MAX_LOADED_ROWS.toLocaleString(
                                        locale,
                                    ),
                                })}
                            </p>
                        )}
                    </>
                ) : undefined
            }
        />
    );
}
