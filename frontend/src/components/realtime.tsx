'use client';

import { Centrifuge, UnauthorizedError } from 'centrifuge';
import { useRouter } from 'next/navigation';
import {
    createContext,
    useContext,
    useEffect,
    useRef,
    useState,
    useSyncExternalStore,
} from 'react';
import {
    realtimeConnectionToken,
    realtimeSubscriptionToken,
} from '@/app/(panel)/realtime-actions';
import type { ActionResult } from '@/lib/types';

/** An event published by the API; see internal/modules/realtime/events.go. */
export type RealtimeEvent = {
    type: string;
    v: number;
    [key: string]: unknown;
};

export type RealtimeState = 'connecting' | 'connected' | 'disconnected';

type Listener = (channel: string, event: RealtimeEvent) => void;

type Ctx = {
    subscribe: (fn: Listener) => () => void;
    getState: () => RealtimeState;
    onState: (fn: () => void) => () => void;
};

const RealtimeContext = createContext<Ctx | null>(null);

/** Codes for which retrying cannot help: Centrifugo stops asking. */
const UNAUTHORIZED = new Set([
    'UNAUTHORIZED',
    'SESSION_INVALID',
    'FORBIDDEN',
    'CHANNEL_FORBIDDEN',
    'MFA_SETUP_REQUIRED',
]);

async function unwrap(r: Promise<ActionResult<string>>): Promise<string> {
    const res = await r;
    if (res.ok) return res.data;
    if (UNAUTHORIZED.has(res.code)) throw new UnauthorizedError(res.message);
    throw new Error(res.message); // temporary: the SDK retries with backoff
}

/** Groups bursts of events (or reconnects) into one page refresh. */
function debounce(fn: () => void, ms: number) {
    let t: ReturnType<typeof setTimeout> | undefined;
    const run = () => {
        clearTimeout(t);
        t = setTimeout(fn, ms);
    };
    run.cancel = () => clearTimeout(t);
    return run;
}

/**
 * One Centrifugo connection for the whole panel. It connects through the
 * same origin (`/connection/websocket`, routed by Caddy), subscribes to the
 * given channels with recovery, and reconnects by itself with backoff, for
 * example after Centrifugo restarts. Whenever missed events could not be
 * recovered, the current page is re-rendered from the API so nothing shown
 * is stale.
 */
export function RealtimeProvider({
    channels,
    enabled = true,
    children,
}: {
    channels: string[];
    /** False while the session may not use the API (pending 2FA set-up). */
    enabled?: boolean;
    children: React.ReactNode;
}) {
    const router = useRouter();
    const listeners = useRef(new Set<Listener>());
    const stateListeners = useRef(new Set<() => void>());
    const state = useRef<RealtimeState>('connecting');
    const key = channels.join(',');

    const [ctx] = useState<Ctx>(() => ({
        subscribe: (fn) => {
            listeners.current.add(fn);
            return () => listeners.current.delete(fn);
        },
        getState: () => state.current,
        onState: (fn) => {
            stateListeners.current.add(fn);
            return () => stateListeners.current.delete(fn);
        },
    }));

    useEffect(() => {
        const setState = (s: RealtimeState) => {
            state.current = s;
            stateListeners.current.forEach((fn) => fn());
        };
        if (!enabled) {
            setState('disconnected');
            return;
        }
        const emit = (channel: string, data: unknown) => {
            if (!data || typeof data !== 'object') return;
            const event = data as RealtimeEvent;
            listeners.current.forEach((fn) => fn(channel, event));
        };
        const resync = debounce(() => router.refresh(), 300);

        const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
        const client = new Centrifuge(
            `${proto}://${window.location.host}/connection/websocket`,
            {
                getToken: () => unwrap(realtimeConnectionToken()),
                name: 'messora-panel',
                maxReconnectDelay: 10_000,
            },
        );
        client.on('connecting', () => setState('connecting'));
        client.on('connected', () => setState('connected'));
        client.on('disconnected', () => setState('disconnected'));
        // Server-side subscriptions: the personal channel and notices.
        client.on('publication', (c) => emit(c.channel, c.data));
        client.on('subscribed', (c) => {
            if (c.wasRecovering && !c.recovered) resync();
        });

        for (const channel of key ? key.split(',') : []) {
            const sub = client.newSubscription(channel, {
                getToken: (c) => unwrap(realtimeSubscriptionToken(c.channel)),
            });
            sub.on('publication', (c) => emit(c.channel, c.data));
            sub.on('subscribed', (c) => {
                // After a reconnect the server replays what was missed;
                // when it cannot (history expired, Centrifugo restarted),
                // fetch the current state instead.
                if (c.wasRecovering && !c.recovered) resync();
            });
            sub.subscribe();
        }
        client.connect();
        return () => {
            resync.cancel();
            client.disconnect();
            for (const sub of Object.values(client.subscriptions())) {
                client.removeSubscription(sub);
            }
        };
    }, [enabled, key, router]);

    return (
        <RealtimeContext.Provider value={ctx}>
            {children}
        </RealtimeContext.Provider>
    );
}

function useCtx(): Ctx {
    const ctx = useContext(RealtimeContext);
    if (!ctx) throw new Error('realtime hooks need a RealtimeProvider');
    return ctx;
}

/** Calls fn for every event the panel receives. */
export function useRealtimeEvents(fn: Listener) {
    const { subscribe } = useCtx();
    const ref = useRef(fn);
    useEffect(() => {
        ref.current = fn;
    });
    useEffect(() => subscribe((c, e) => ref.current(c, e)), [subscribe]);
}

/** The connection state, for status indicators. */
export function useRealtimeState(): RealtimeState {
    const { getState, onState } = useCtx();
    return useSyncExternalStore(onState, getState, () => 'connecting');
}

/**
 * Re-renders the current page when a matching event arrives, so server
 * components show new data without polling.
 */
export function RealtimeRefresh({
    channel,
    types,
}: {
    channel: string;
    types?: string[];
}) {
    const router = useRouter();
    const refresh = useRef(debounce(() => router.refresh(), 500));
    useEffect(() => {
        const r = refresh.current;
        return () => r.cancel();
    }, []);
    useRealtimeEvents((c, e) => {
        if (c === channel && (!types || types.includes(e.type))) {
            refresh.current();
        }
    });
    return null;
}
