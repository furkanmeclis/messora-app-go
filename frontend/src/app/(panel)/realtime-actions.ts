'use server';

import { api } from '@/lib/server/api';
import { runAction } from '@/lib/server/actions';
import type { ActionResult, Schemas } from '@/lib/types';

/*
 * Centrifugo tokens for the browser. The Go API signs them with the secret
 * it shares with Centrifugo; the BFF only relays them on the user's own
 * session, so neither the session token nor any secret reaches the browser.
 */

export async function realtimeConnectionToken(): Promise<ActionResult<string>> {
    return runAction(async () => {
        const r = await api<Schemas['RealtimeToken']>(
            '/realtime/connection-token',
            { method: 'POST' },
        );
        return r.data.token;
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
