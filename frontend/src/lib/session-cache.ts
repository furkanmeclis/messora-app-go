/**
 * A small in-process cache of per-session values (the signed-in user), so
 * the server renders behind one navigation — the page, its layouts and the
 * prefetches of the links it shows — share one API call instead of costing
 * one each against the session's rate-limit budget.
 *
 * Only successful values are stored; failures always reach the API again.
 * Concurrent loads of the same key share one request. Entries expire after
 * `ttlMs`, and `invalidate` drops a session's entry (every change the BFF
 * makes for that session does, see api.ts).
 */
export type SessionCache<T> = {
    get(key: string, load: () => Promise<T>): Promise<T>;
    invalidate(key: string): void;
    clear(): void;
    readonly size: number;
};

export function sessionCache<T>(
    ttlMs: number,
    maxEntries: number,
    now: () => number = Date.now,
): SessionCache<T> {
    const values = new Map<string, { at: number; value: T }>();
    const inflight = new Map<string, Promise<T>>();
    // Bumped by invalidate, so a load that started before a change does
    // not store the value it read.
    const generation = new Map<string, number>();

    return {
        async get(key, load) {
            const hit = values.get(key);
            if (hit && now() - hit.at < ttlMs) return hit.value;
            const running = inflight.get(key);
            if (running) return running;
            const gen = generation.get(key) ?? 0;
            const p = load().then(
                (value) => {
                    inflight.delete(key);
                    if ((generation.get(key) ?? 0) === gen) {
                        if (values.size >= maxEntries) values.clear();
                        values.set(key, { at: now(), value });
                    }
                    return value;
                },
                (err: unknown) => {
                    inflight.delete(key);
                    throw err;
                },
            );
            inflight.set(key, p);
            return p;
        },
        invalidate(key) {
            values.delete(key);
            inflight.delete(key);
            if (generation.size >= maxEntries) generation.clear();
            generation.set(key, (generation.get(key) ?? 0) + 1);
        },
        clear() {
            values.clear();
            inflight.clear();
            generation.clear();
        },
        get size() {
            return values.size;
        },
    };
}
