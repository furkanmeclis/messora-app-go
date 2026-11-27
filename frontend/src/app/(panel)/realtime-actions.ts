'use server';

import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import type { ActionResult, Schemas } from '@/lib/types';

/*
 * Centrifugo tokens for the browser. The Go API signs them with the secret
 * it shares with Centrifugo; the BFF only relays them on the user's own
 * session, so neither the session token nor any secret reaches the browser.
 */

/**
 * A connection token and the prefix of the workspace's channel names
 * (`tenant:<tenant_id>:`): the browser subscribes to prefix + channel.
 */
export async function realtimeConnectionToken(): Promise<
    ActionResult<{ token: string; channelPrefix: string }>
> {
    return runAction(async () => {
        const r = await api<Schemas['RealtimeToken']>(
            '/realtime/connection-token',
            { method: 'POST' },
        );
        return {
            token: r.data.token,
            channelPrefix: r.data.channel_prefix ?? '',
        };
    });
}

export async function realtimeSubscriptionToken(
    channel: string,
): Promise<ActionResult<string>> {
    return runAction(async () => {
        const r = await api<Schemas['RealtimeToken']>(
            '/realtime/subscription-token',
            { method: 'POST', body: { channel } },
        );
        return r.data.token;
    });
}

/**
 * What changed on a channel since a moment, for a client whose missed
 * events could not be recovered from Centrifugo's history.
 */
export async function realtimeSync(
    channel: string,
    since: string,
): Promise<ActionResult<Schemas['RealtimeSync']>> {
    return runAction(async () => {
        const q = new URLSearchParams({ channel, since });
        const r = await api<Schemas['RealtimeSync']>(`/realtime/sync?${q}`);
        return r.data;
    });
}
