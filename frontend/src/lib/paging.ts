/**
 * Paging helpers shared by the tables: the numbered pager and a loader that
 * walks a cursor-paged API on demand (ClientDataTable over lists the API
 * hands out in chunks). Free of React so they can be unit-tested.
 */

/** Rows a table loads at most from a cursor-paged API; filters and exports go further. */
export const MAX_LOADED_ROWS = 5000;

/**
 * The page buttons: first, last and the neighbours of the current page with
 * "…" for gaps. `more` (rows beyond the last known page exist) ends the list
 * with "…" instead of pretending the last known page is the last one.
 */
export function pageNumbers(
    page: number,
    pages: number,
    more = false,
): (number | '…')[] {
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
    if (more) out.push('…');
    return out;
}

/**
 * How many rows the table needs loaded: the page being shown plus the next
 * one (so "next" works without waiting), or everything up to the cap when
 * the whole list matters (last page, a search or a different sort order).
 */
export function rowsWanted({
    page,
    pageSize,
    wantAll,
    cap = MAX_LOADED_ROWS,
}: {
    page: number;
    pageSize: number;
    wantAll: boolean;
    cap?: number;
}): number {
    if (wantAll || !Number.isFinite(page)) return cap;
    return Math.min(cap, (page + 1) * pageSize);
}

export type CursorPage<T> = { items: T[]; next?: string };

export type CursorState<T> = {
    rows: T[];
    /** Cursor of the next chunk; undefined once everything is loaded. */
    next?: string;
    loading: boolean;
    /** The cap stopped loading while the API still had rows. */
    capped: boolean;
};

/**
 * Holds the rows of a cursor-paged list and loads further chunks when asked
 * for a row count (`ensure`). One request runs at a time; asking again while
 * it runs only raises the target. `reset` (a new filter, a refresh) drops
 * the answers of requests that were still running.
 */
export class CursorLoader<T> {
    private state: CursorState<T>;
    private fetchPage: (cursor: string) => Promise<CursorPage<T> | null>;
    private readonly rowId: (row: T) => string;
    private readonly cap: number;
    private readonly onChange: (s: CursorState<T>) => void;
    private target = 0;
    private running: Promise<void> | null = null;
    private generation = 0;

    constructor(opts: {
        rows: T[];
        next?: string;
        /** Loads the chunk after cursor; null on failure (stops loading). */
        fetchPage: (cursor: string) => Promise<CursorPage<T> | null>;
        rowId: (row: T) => string;
        cap?: number;
        onChange: (s: CursorState<T>) => void;
    }) {
        this.fetchPage = opts.fetchPage;
        this.rowId = opts.rowId;
        this.cap = opts.cap ?? MAX_LOADED_ROWS;
        this.onChange = opts.onChange;
        this.state = this.settle({
            rows: opts.rows,
            next: opts.next,
            loading: false,
            capped: false,
        });
    }

    /** Swaps the loader of later chunks (it closes over newer props). */
    setFetchPage(fn: (cursor: string) => Promise<CursorPage<T> | null>) {
        this.fetchPage = fn;
    }

    get current(): CursorState<T> {
        return this.state;
    }

    private settle(s: CursorState<T>): CursorState<T> {
        return { ...s, capped: Boolean(s.next) && s.rows.length >= this.cap };
    }

    private set(patch: Partial<CursorState<T>>) {
        this.state = this.settle({ ...this.state, ...patch });
        this.onChange(this.state);
    }

    /** Loads chunks until n rows are loaded, the list ends or the cap is hit. */
    ensure(n: number): Promise<void> {
        this.target = Math.max(this.target, Math.min(n, this.cap));
        if (this.running) return this.running;
        if (!this.state.next || this.state.rows.length >= this.target)
            return Promise.resolve();
        const gen = this.generation;
        this.running = (async () => {
            this.set({ loading: true });
            try {
                while (
                    gen === this.generation &&
                    this.state.next &&
                    this.state.rows.length < this.target
                ) {
                    const page = await this.fetchPage(this.state.next);
                    if (gen !== this.generation) return;
                    if (!page) {
                        // A failure stops here; asking again retries.
                        this.target = 0;
                        break;
                    }
                    const seen = new Set(this.state.rows.map(this.rowId));
                    this.set({
                        rows: [
                            ...this.state.rows,
                            ...page.items.filter(
                                (r) => !seen.has(this.rowId(r)),
                            ),
                        ],
                        // A chunk without rows cannot advance: stop there.
                        next: page.items.length > 0 ? page.next : undefined,
                    });
                }
            } finally {
                if (gen === this.generation) {
                    this.running = null;
                    this.set({ loading: false });
                }
            }
        })();
        return this.running;
    }

    /** Starts over with a fresh first chunk (a filter change, a refresh). */
    reset(rows: T[], next?: string) {
        this.generation++;
        this.running = null;
        this.target = 0;
        this.set({ rows, next, loading: false });
    }

    /**
     * Replaces the first chunk (a server refresh of the first page) and keeps
     * the rows loaded after it that it does not repeat.
     */
    rebase(first: T[], previousFirst: T[]) {
        const firstIds = new Set(first.map(this.rowId));
        const oldIds = new Set(previousFirst.map(this.rowId));
        this.set({
            rows: [
                ...first,
                ...this.state.rows.filter((r) => {
                    const id = this.rowId(r);
                    return !firstIds.has(id) && !oldIds.has(id);
                }),
            ],
        });
    }

    /** Changes loaded rows in place (realtime updates, a redelivery). */
    update(fn: (row: T) => T) {
        this.set({ rows: this.state.rows.map(fn) });
    }
}
