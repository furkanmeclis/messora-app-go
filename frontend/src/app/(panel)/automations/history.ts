'use client';

import { useCallback, useReducer, useRef } from 'react';

const LIMIT = 100;
/** Edits of the same thing within this time are one undo step (typing). */
const COALESCE_MS = 1200;

type Stacks<T> = { present: T; past: T[]; future: T[] };

type Action<T> =
    | { type: 'set'; fn: (s: T) => T; record: boolean }
    | { type: 'checkpoint' }
    | { type: 'undo' | 'redo'; restore: (snapshot: T, current: T) => T };

function reducer<T>(s: Stacks<T>, a: Action<T>): Stacks<T> {
    switch (a.type) {
        case 'set': {
            const present = a.fn(s.present);
            if (present === s.present) return s;
            if (!a.record) return { ...s, present };
            return {
                present,
                past: [...s.past.slice(-(LIMIT - 1)), s.present],
                future: [],
            };
        }
        case 'checkpoint':
            return {
                ...s,
                past: [...s.past.slice(-(LIMIT - 1)), s.present],
                future: [],
            };
        case 'undo': {
            const prev = s.past.at(-1);
            if (prev === undefined) return s;
            return {
                present: a.restore(prev, s.present),
                past: s.past.slice(0, -1),
                future: [s.present, ...s.future],
            };
        }
        case 'redo': {
            const [next, ...rest] = s.future;
            if (next === undefined) return s;
            return {
                present: a.restore(next, s.present),
                past: [...s.past, s.present],
                future: rest,
            };
        }
    }
}

export type History<T> = {
    state: T;
    /**
     * Changes the state. Pass `record: false` for transient updates (node
     * measurements, selection, a drag in progress) and a `group` key to
     * merge quick successive edits of the same field into one undo step.
     */
    set: (fn: (s: T) => T, opts?: { record?: boolean; group?: string }) => void;
    /** Records the current state as an undo step (e.g. before a drag). */
    checkpoint: () => void;
    undo: () => void;
    redo: () => void;
    canUndo: boolean;
    canRedo: boolean;
};

/**
 * State with undo and redo. `restore` adapts a snapshot when it comes back
 * (e.g. keeps the current node measurements and selection).
 */
export function useHistory<T>(
    initial: () => T,
    restore: (snapshot: T, current: T) => T = (s) => s,
): History<T> {
    const [stacks, dispatch] = useReducer(
        reducer<T>,
        undefined,
        (): Stacks<T> => ({ present: initial(), past: [], future: [] }),
    );
    const group = useRef<{ key: string; at: number } | null>(null);

    const set = useCallback<History<T>['set']>((fn, opts = {}) => {
        let record = opts.record ?? true;
        if (record) {
            const now = Date.now();
            const g = group.current;
            if (
                opts.group !== undefined &&
                g?.key === opts.group &&
                now - g.at < COALESCE_MS
            ) {
                record = false; // same undo step as the previous edit
            }
            group.current = opts.group ? { key: opts.group, at: now } : null;
        }
        dispatch({ type: 'set', fn, record });
    }, []);

    const checkpoint = useCallback(() => {
        group.current = null;
        dispatch({ type: 'checkpoint' });
    }, []);
    const undo = useCallback(() => {
        group.current = null;
        dispatch({ type: 'undo', restore });
    }, [restore]);
    const redo = useCallback(() => {
        group.current = null;
        dispatch({ type: 'redo', restore });
    }, [restore]);

    return {
        state: stacks.present,
        set,
        checkpoint,
        undo,
        redo,
        canUndo: stacks.past.length > 0,
        canRedo: stacks.future.length > 0,
    };
}
