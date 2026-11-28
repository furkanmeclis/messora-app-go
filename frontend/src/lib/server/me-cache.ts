import 'server-only';

import { createHash } from 'node:crypto';
import { sessionCache } from '@/lib/session-cache';
import type { Me } from '@/lib/types';

/**
 * `/me` for a few seconds per session and host. React's cache() already
 * shares it within one server render, but one navigation is several
 * renders (the page plus the prefetches of the links it shows), and each
 * used to cost a `/me` call against the session's API budget. Five seconds
 * cover one navigation; anything the panel changes for the session drops
 * the entry at once (api.ts), so the user never sees their own change
 * late. A change someone else makes (an admin changing this user's role)
 * shows within the TTL; the API enforces permissions on every call anyway.
 */
export const ME_TTL_MS = 5_000;
const MAX_ENTRIES = 1_000;

export const meCache = sessionCache<Me>(ME_TTL_MS, MAX_ENTRIES);

/** The cache key of a session token at a host (never the token itself). */
export function meKey(token: string, host: string | null): string {
    return createHash('sha256')
        .update(`${host ?? ''}\n${token}`)
        .digest('hex');
}
