'use client';

import { useMemo, useState } from 'react';
import { Search } from 'lucide-react';
import {
    DataTable,
    type Column,
    type Selection,
} from '@/components/data-table';
import type { MenuGroup } from '@/components/context-menu';
import { Input } from '@/components/ui/input';
import { useLocale } from '@/i18n/client';
import { intlLocale } from '@/i18n/config';

export type ClientColumn<T> = Omit<Column<T>, 'sort'> & {
    /** Makes the column sortable by this value. */
    sortValue?: (row: T) => string | number | Date | null | undefined;
};

/**
 * DataTable for lists the page already holds in full: sorting, paging and
 * search happen in the browser; selection, bulk actions, column visibility,
 * density and the row menu work as in server-paged tables.
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
}) {
    const locale = intlLocale(useLocale());
    const [q, setQ] = useState('');
    const [sort, setSort] = useState(initialSort ?? '');
    const [page, setPage] = useState(1);
    const [pageSize, setPageSize] = useState(initialPageSize);

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
            onNavigate={(patch) => {
                if ('sort' in patch) setSort(String(patch.sort ?? ''));
                if ('limit' in patch && patch.limit)
                    setPageSize(Number(patch.limit));
                if ('page' in patch)
                    setPage(patch.page ? Number(patch.page) : 1);
            }}
            toolbar={
                searchText || toolbar ? (
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
                        {toolbar}
                    </>
                ) : undefined
            }
        />
    );
}
