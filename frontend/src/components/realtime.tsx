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
    realtimeSync,
} from '@/app/(panel)/realtime-actions';
import {
    ChannelPositions,
    EventDeduper,
    planResync,
    type Position,
} from '@/lib/realtime-sync';
import type { ActionResult } from '@/lib/types';

/**
 * An event published by the API; see internal/modules/realtime/events.go.
 * Every event has `type` (`<area>.<event>`), `v`, a unique `id` and `ts`.
 */
export type RealtimeEvent = {
    type: string;
    v: number;
    id?: string;
    ts?: string;
    /** Set on snapshots fetched by a re-sync instead of pushed live. */
    synced?: boolean;
    [key: string]: unknown;
};

/**
 * Client-side event emitted on a channel after it was re-synced through
 * GET /realtime/sync (missed events could not be recovered); pages whose
 * lists may have lost items re-render on it.
 */
export const RESYNCED = 'realtime.resynced';

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
 * example after Centrifugo restarts. Copies of an event (same `id`) are
 * delivered once. Whenever missed events could not be recovered, the
 * channel is re-synced through GET /realtime/sync (snapshots arrive as
 * ordinary events, then RESYNCED); after a long gap, or when too much
 * changed, the current page is re-rendered from the API instead.
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
        const dedupe = new EventDeduper(2000);
        const positions = new ChannelPositions();
        const emit = (channel: string, data: unknown) => {
            if (!data || typeof data !== 'object') return;
            const event = data as RealtimeEvent;
            // One event published to several channels (inbox:all and the
            // assignee's personal channel) arrives once.
            if (dedupe.seen(event.id)) return;
            listeners.current.forEach((fn) => fn(channel, event));
        };
        const reload = debounce(() => router.refresh(), 300);
        // After a reconnect the server replays what was missed; when it
        // cannot (history expired, Centrifugo restarted), fetch the current
        // state of what the channel announces since it was last current,
        // and reload the page only when that is too much or too long ago.
        const resync = async (channel: string, last: Position | undefined) => {
            if (channel.startsWith('system:')) return;
            const plan = planResync(last, Date.now());
            if (plan.kind === 'reload') return reload();
            const res = await realtimeSync(channel, plan.since).catch(
                () => null,
            );
            if (!res?.ok || res.data.has_more) return reload();
            for (const e of res.data.events) {
                emit(channel, { ...e, synced: true });
            }
            emit(channel, { type: RESYNCED, v: 1, synced: true });
        };
        const onSubscribed = (c: {
            channel: string;
            wasRecovering: boolean;
            recovered: boolean;
            streamPosition?: { offset: number; epoch: string };
        }) => {
            const last = positions.get(c.channel);
            if (c.wasRecovering && !c.recovered) void resync(c.channel, last);
            positions.mark(c.channel, Date.now(), c.streamPosition);
        };
        const onPublication = (c: {
            channel: string;
            data: unknown;
            offset?: number;
        }) => {
            positions.mark(c.channel, Date.now(), { offset: c.offset });
            emit(c.channel, c.data);
        };

        const proto = window.location.protocol === 'https:' ? 'wss' : 'ws';
        const client = new Centrifuge(
            `${proto}://${window.location.host}/connection/websocket`,
            {
                getToken: () => unwrap(realtimeConnectionToken()),
                name: 'messora-panel',
                maxReconnectDelay: 10_000,
            },
        );
        const serverChannels = new Set<string>();
        client.on('connecting', () => setState('connecting'));
        client.on('connected', () => setState('connected'));
        client.on('disconnected', () => setState('disconnected'));
        // Server-side subscriptions: the personal channel and notices.
        client.on('publication', onPublication);
        client.on('subscribed', (c) => {
            serverChannels.add(c.channel);
            onSubscribed(c);
        });

        for (const channel of key ? key.split(',') : []) {
            const sub = client.newSubscription(channel, {
                getToken: (c) => unwrap(realtimeSubscriptionToken(c.channel)),
            });
            sub.on('publication', onPublication);
            sub.on('subscribed', onSubscribed);
            sub.subscribe();
        }
        // While connected, every subscribed channel is current. Timers stop
        // while the device sleeps, so the last mark bounds what was missed.
        const heartbeat = setInterval(() => {
            if (client.state !== 'connected') return;
            const now = Date.now();
            serverChannels.forEach((ch) => positions.mark(ch, now));
            for (const sub of Object.values(client.subscriptions())) {
                if (sub.state === 'subscribed')
                    positions.mark(sub.channel, now);
            }
        }, 30_000);
        client.connect();
        return () => {
            clearInterval(heartbeat);
            reload.cancel();
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

/** A page refresh that groups calls within `ms` into one. */
export function useDebouncedRefresh(ms = 500): () => void {
    const router = useRouter();
    const [refresh] = useState(() => debounce(() => router.refresh(), ms));
    useEffect(() => () => refresh.cancel(), [refresh]);
    return refresh;
}

/**
 * Re-renders the current page when a matching event arrives on one of the
 * channels, so server components show new data without polling. A re-sync
 * of a channel (RESYNCED) counts as a match.
 */
export function RealtimeRefresh({
    channel,
    types,
}: {
    channel: string | string[];
    types?: string[];
}) {
    const refresh = useDebouncedRefresh();
    useRealtimeEvents((c, e) => {
        const match = Array.isArray(channel)
            ? channel.includes(c)
            : c === channel;
        if (
            match &&
            (!types || types.includes(e.type) || e.type === RESYNCED)
        ) {
            refresh();
        }
    });
    return null;
}
