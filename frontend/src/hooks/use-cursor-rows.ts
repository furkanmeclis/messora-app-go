'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { CursorLoader, type CursorPage, type CursorState } from '@/lib/paging';

/**
 * What a ClientDataTable needs to page through rows that are not all loaded
 * yet: its own pager then asks for more rows instead of a separate
 * "load more" button.
 */
export type TablePaging = {
    /** The API has rows beyond the loaded ones. */
    hasMore: boolean;
    loading: boolean;
    /** Loading stopped at the row cap while more rows exist. */
    capped: boolean;
    /** Load until n rows are there (or the list ends, or the cap). */
    ensure: (n: number) => void;
};

/**
 * The rows of a cursor-paged list, loaded chunk by chunk as the table needs
 * them. `initial` is the first chunk the server rendered; when it changes (a
 * refresh) it replaces the first chunk and later chunks stay.
 */
export function useCursorRows<T>({
    initial,
    initialCursor,
    fetchPage,
    rowId,
}: {
    initial: T[];
    initialCursor?: string;
    /** The chunk after cursor; null when it failed (already reported). */
    fetchPage: (cursor: string) => Promise<CursorPage<T> | null>;
    rowId: (row: T) => string;
}) {
    const [state, setState] = useState<CursorState<T> | null>(null);
    const [loader] = useState(
        () =>
            new CursorLoader<T>({
                rows: initial,
                next: initialCursor,
                fetchPage,
                rowId,
                onChange: setState,
            }),
    );
    const [first, setFirst] = useState(initial);
    if (first !== initial) {
        setFirst(initial);
        loader.rebase(initial, first);
    }
    const current = state ?? loader.current;
    useEffect(() => loader.setFetchPage(fetchPage), [loader, fetchPage]);

    const ensure = useCallback((n: number) => void loader.ensure(n), [loader]);
    const paging = useMemo<TablePaging>(
        () => ({
            hasMore: Boolean(current.next),
            loading: current.loading,
            capped: current.capped,
            ensure,
        }),
        [current.next, current.loading, current.capped, ensure],
    );
    const update = useCallback(
        (fn: (row: T) => T) => loader.update(fn),
        [loader],
    );
    const reset = useCallback(
        (rows: T[], next?: string) => loader.reset(rows, next),
        [loader],
    );
    return { rows: current.rows, paging, update, reset };
}
