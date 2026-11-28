import 'server-only';

import { createHash } from 'node:crypto';
import { api } from './api';
import { getSessionToken } from './session';
import type { Membership } from '@/lib/auth/contracts';

/**
 * The signed-in person's workspaces for the switcher. Every panel page
 * renders the switcher, so the list is kept for a minute per session
 * instead of costing one API call (and rate-limit budget) per page. A
 * stale list is harmless: switching asks the API again.
 */
const TTL_MS = 60_000;
const MAX_ENTRIES = 1_000;
const cache = new Map<string, { at: number; list: Membership[] }>();

export async function myMemberships(): Promise<Membership[]> {
    const token = await getSessionToken();
    if (!token) return [];
    const key = createHash('sha256').update(token).digest('hex');
    const now = Date.now();
    const hit = cache.get(key);
    if (hit && now - hit.at < TTL_MS) return hit.list;
    const list = await api<Membership[]>('/me/memberships')
        .then((r) => r.data)
        .catch(() => null);
    if (!list) return hit?.list ?? [];
    if (cache.size >= MAX_ENTRIES) cache.clear();
    cache.set(key, { at: now, list });
    return list;
}
