import { test } from 'node:test';
import assert from 'node:assert/strict';
import {
    CursorLoader,
    pageNumbers,
    rowsWanted,
    type CursorPage,
    type CursorState,
} from './paging.ts';

type Row = { id: string };
const rows = (from: number, n: number): Row[] =>
    Array.from({ length: n }, (_, i) => ({ id: String(from + i) }));

/** An API of total rows handed out in chunks of size. */
function api(total: number, size: number) {
    const calls: string[] = [];
    const fetchPage = async (cursor: string): Promise<CursorPage<Row>> => {
        calls.push(cursor);
        const from = Number(cursor);
        const end = Math.min(total, from + size);
        return {
            items: rows(from, end - from),
            next: end < total ? String(end) : undefined,
        };
    };
    return { calls, fetchPage };
}

function loader(total: number, size: number, cap?: number) {
    const a = api(total, size);
    const states: CursorState<Row>[] = [];
    const l = new CursorLoader<Row>({
        rows: rows(0, Math.min(size, total)),
        next: total > size ? String(size) : undefined,
        fetchPage: a.fetchPage,
        rowId: (r) => r.id,
        cap,
        onChange: (s) => states.push(s),
    });
    return { l, calls: a.calls, states };
}

test('page numbers: gaps collapse to …; more rows end the list with …', () => {
    assert.deepEqual(pageNumbers(1, 1), [1]);
    assert.deepEqual(pageNumbers(1, 3), [1, 2, 3]);
    assert.deepEqual(pageNumbers(5, 10), [1, '…', 4, 5, 6, '…', 10]);
    assert.deepEqual(pageNumbers(2, 2, true), [1, 2, '…']);
    assert.deepEqual(pageNumbers(1, 4, true), [1, 2, '…', 4, '…']);
});

test('rows wanted: the shown page plus the next, everything up to the cap when asked', () => {
    assert.equal(rowsWanted({ page: 1, pageSize: 25, wantAll: false }), 50);
    assert.equal(rowsWanted({ page: 4, pageSize: 50, wantAll: false }), 250);
    assert.equal(
        rowsWanted({ page: 1, pageSize: 25, wantAll: true, cap: 1000 }),
        1000,
    );
    assert.equal(
        rowsWanted({ page: Infinity, pageSize: 25, wantAll: false, cap: 300 }),
        300,
    );
    assert.equal(
        rowsWanted({ page: 400, pageSize: 25, wantAll: false, cap: 300 }),
        300,
    );
});

test('ensure loads chunks until enough rows are there', async () => {
    const { l, calls } = loader(200, 25);
    await l.ensure(60);
    assert.equal(l.current.rows.length, 75);
    assert.deepEqual(calls, ['25', '50']);
    assert.equal(l.current.next, '75');
    assert.equal(l.current.loading, false);
    // Already loaded: no request.
    await l.ensure(70);
    assert.equal(calls.length, 2);
});

test('ensure stops at the end of the list', async () => {
    const { l, calls } = loader(60, 25);
    await l.ensure(1000);
    assert.equal(l.current.rows.length, 60);
    assert.equal(l.current.next, undefined);
    assert.equal(l.current.capped, false);
    assert.equal(calls.length, 2);
});

test('the cap stops loading and is reported while more rows exist', async () => {
    const { l } = loader(500, 50, 120);
    await l.ensure(Infinity);
    assert.equal(l.current.rows.length, 150);
    assert.equal(l.current.capped, true);
    assert.ok(l.current.next);
});

test('concurrent ensure calls share one request loop and raise the target', async () => {
    const { l, calls } = loader(300, 25);
    const a = l.ensure(30);
    const b = l.ensure(100);
    assert.equal(a, b);
    await a;
    assert.equal(l.current.rows.length, 100);
    assert.deepEqual(calls, ['25', '50', '75']);
});

test('reset drops answers of requests still running', async () => {
    let release: (p: CursorPage<Row>) => void = () => {};
    const l = new CursorLoader<Row>({
        rows: rows(0, 2),
        next: 'a',
        fetchPage: () => new Promise((r) => (release = r)),
        rowId: (r) => r.id,
        onChange: () => {},
    });
    const p = l.ensure(10);
    assert.equal(l.current.loading, true);
    l.reset(rows(100, 1));
    release({ items: rows(2, 5), next: 'b' });
    await p;
    assert.deepEqual(
        l.current.rows.map((r) => r.id),
        ['100'],
    );
    assert.equal(l.current.loading, false);
});

test('a failed chunk stops loading; asking again retries', async () => {
    let fail = true;
    const l = new CursorLoader<Row>({
        rows: rows(0, 2),
        next: '2',
        fetchPage: async () =>
            fail ? null : { items: rows(2, 2), next: undefined },
        rowId: (r) => r.id,
        onChange: () => {},
    });
    await l.ensure(10);
    assert.equal(l.current.rows.length, 2);
    assert.equal(l.current.next, '2');
    fail = false;
    await l.ensure(10);
    assert.equal(l.current.rows.length, 4);
});

test('duplicates across chunks are dropped; an empty chunk ends loading', async () => {
    const l = new CursorLoader<Row>({
        rows: rows(0, 3),
        next: 'x',
        fetchPage: async (c) =>
            c === 'x'
                ? { items: rows(2, 3), next: 'y' }
                : { items: [], next: 'z' },
        rowId: (r) => r.id,
        onChange: () => {},
    });
    await l.ensure(100);
    assert.deepEqual(
        l.current.rows.map((r) => r.id),
        ['0', '1', '2', '3', '4'],
    );
    assert.equal(l.current.next, undefined);
});

test('rebase swaps the first chunk and keeps later rows', () => {
    const l = new CursorLoader<Row>({
        rows: rows(0, 4),
        next: '4',
        fetchPage: async () => null,
        rowId: (r) => r.id,
        onChange: () => {},
    });
    // The first chunk was rows 0-1; a refresh brings a new row 9 and 0-1.
    l.rebase([{ id: '9' }, ...rows(0, 2)], rows(0, 2));
    assert.deepEqual(
        l.current.rows.map((r) => r.id),
        ['9', '0', '1', '2', '3'],
    );
    l.update((r) => (r.id === '2' ? { id: '2b' } : r));
    assert.equal(l.current.rows[3]!.id, '2b');
});
