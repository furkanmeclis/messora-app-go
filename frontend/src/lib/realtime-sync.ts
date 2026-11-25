/**
 * Pure helpers of the realtime client (components/realtime.tsx): duplicate
 * elimination and per-channel stream positions, kept free of React and the
 * Centrifugo SDK so they can be unit-tested.
 */

/** Remembers the last `max` event IDs; older ones are forgotten first. */
export class EventDeduper {
    private readonly ids = new Set<string>();
    private readonly max: number;

    constructor(max = 1000) {
        this.max = max;
    }

    /**
     * True when id was seen before. An event published to several channels
     * (inbox:all and a personal channel, say) keeps one ID, so the second
     * copy is dropped. Events without an ID are never treated as copies.
     */
    seen(id: unknown): boolean {
        if (typeof id !== 'string' || id === '') return false;
        if (this.ids.has(id)) {
            // Refresh its position: recently repeated IDs stay longest.
            this.ids.delete(id);
            this.ids.add(id);
            return true;
        }
        this.ids.add(id);
        if (this.ids.size > this.max) {
            const oldest = this.ids.values().next().value;
            if (oldest !== undefined) this.ids.delete(oldest);
        }
        return false;
    }

    get size(): number {
        return this.ids.size;
    }
}

/** Where a channel's stream was last seen. */
export type Position = {
    offset?: number;
    epoch?: string;
    /** Epoch ms of the last moment the channel was known to be current. */
    at: number;
};

/** Per-channel stream positions (Centrifugo offset/epoch) and times. */
export class ChannelPositions {
    private readonly map = new Map<string, Position>();

    /** Records that channel is current at `at` (and its offset/epoch). */
    mark(
        channel: string,
        at: number,
        pos: { offset?: number | bigint; epoch?: string } = {},
    ): void {
        const prev = this.map.get(channel);
        const offset =
            pos.offset === undefined ? prev?.offset : Number(pos.offset);
        this.map.set(channel, {
            offset,
            epoch: pos.epoch ?? prev?.epoch,
            at: Math.max(at, prev?.at ?? 0),
        });
    }

    get(channel: string): Position | undefined {
        return this.map.get(channel);
    }
}

/** Longest gap re-synced through /realtime/sync; longer ones reload. */
export const MAX_SYNC_GAP_MS = 60 * 60 * 1000;
/** Margin for clock skew between browser and server, and in-flight events. */
export const SYNC_MARGIN_MS = 60 * 1000;

export type ResyncPlan = { kind: 'sync'; since: string } | { kind: 'reload' };

/**
 * Decides how to catch up after events of a channel were lost: fetch what
 * changed since the channel was last current, or reload the page when that
 * moment is unknown or longer ago than MAX_SYNC_GAP_MS.
 */
export function planResync(
    last: Position | undefined,
    now: number,
): ResyncPlan {
    if (!last || now - last.at > MAX_SYNC_GAP_MS) return { kind: 'reload' };
    return {
        kind: 'sync',
        since: new Date(last.at - SYNC_MARGIN_MS).toISOString(),
    };
}

/** Channels that carry inbox activity for the current user. */
export function isInboxChannel(channel: string): boolean {
    return channel.startsWith('inbox:') || channel.startsWith('personal:');
}
