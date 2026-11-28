import 'server-only';

import { headers } from 'next/headers';
import { serverConfig } from './config';
import { getPlatformToken, getSessionToken } from './session';
import { meCache, meKey } from './me-cache';
import { forwardHost, requestHost } from './tenant-host';
import type { ApiErrorBody } from '@/lib/types';

export class ApiError extends Error {
    constructor(
        readonly status: number,
        readonly code: string,
        message: string,
        readonly requestId: string,
        readonly details: ApiErrorBody['details'] = [],
        readonly retryAfter?: number,
    ) {
        super(message);
        this.name = 'ApiError';
    }

    /** Plain object safe to return from a Server Action. */
    toJSON() {
        return {
            status: this.status,
            code: this.code,
            message: this.message,
            requestId: this.requestId,
            details: this.details,
            retryAfter: this.retryAfter,
        };
    }
}

type Envelope<T> = {
    data: T;
    meta: { request_id: string; next_cursor?: string; total?: number };
};
type ErrorEnvelope = { error: ApiErrorBody; meta: { request_id: string } };

export type ApiResult<T> = {
    data: T;
    requestId: string;
    nextCursor?: string;
    /** Matching items, for paged lists. */
    total?: number;
};

type Options = {
    method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
    body?: unknown;
    /**
     * Defaults to the signed-in user's session; 'platform' sends the
     * platform admin's session (admin host only).
     */
    auth?: 'session' | 'none' | 'platform';
    /** A session token to use instead of the cookie (right after sign-in). */
    token?: string;
    idempotencyKey?: string;
    timeoutMs?: number;
};

/**
 * Calls the Go API over the internal network. Every call carries the panel
 * secret, the host the browser opened (the API serves the tenant of that
 * host), the user's session (unless `auth: 'none'`), the browser's address
 * and user agent (for audit logs and rate limits) and a request ID that
 * appears in both services' logs.
 */
export async function api<T>(
    path: string,
    o: Options = {},
): Promise<ApiResult<T>> {
    const { apiInternalUrl, bffSharedSecret } = serverConfig();
    const requestId = crypto.randomUUID();
    const incoming = await headers();
    const h = new Headers({
        Accept: 'application/json',
        'X-Messora-BFF': bffSharedSecret,
        'X-Request-ID': requestId,
    });
    forwardHost(incoming, h);
    const xff = incoming.get('x-forwarded-for');
    if (xff) h.set('X-Forwarded-For', xff.split(',').pop()!.trim());
    const ua = incoming.get('user-agent');
    if (ua) h.set('User-Agent', ua.slice(0, 512));
    let staleMe: string | undefined;
    if (o.auth !== 'none') {
        const token =
            o.token ??
            (o.auth === 'platform'
                ? await getPlatformToken()
                : await getSessionToken());
        if (token) h.set('Authorization', `Bearer ${token}`);
        // Whatever the panel changes for a session (profile, language,
        // 2FA, roles, settings) may change its /me: read it fresh next time.
        // Dropped before and after the call, so a /me read racing with the
        // change cannot keep the old value either.
        if (token && o.auth !== 'platform' && (o.method ?? 'GET') !== 'GET') {
            staleMe = meKey(token, requestHost(incoming));
            meCache.invalidate(staleMe);
        }
    }
    if (o.body !== undefined) h.set('Content-Type', 'application/json');
    if (o.idempotencyKey) h.set('Idempotency-Key', o.idempotencyKey);

    let res: Response;
    try {
        res = await fetch(`${apiInternalUrl}/api/v1${path}`, {
            method: o.method ?? 'GET',
            headers: h,
            body: o.body === undefined ? undefined : JSON.stringify(o.body),
            cache: 'no-store',
            signal: AbortSignal.timeout(o.timeoutMs ?? 15_000),
        });
    } catch {
        if (staleMe) meCache.invalidate(staleMe);
        throw new ApiError(
            503,
            'API_UNREACHABLE',
            'The Messora API is not reachable. Try again in a moment.',
            requestId,
        );
    }
    if (staleMe) meCache.invalidate(staleMe);
    // 204 has no body; 202 may (e.g. a queued message or import).
    if (
        res.status === 204 ||
        (res.status === 202 && res.headers.get('content-length') === '0')
    ) {
        return { data: undefined as T, requestId };
    }
    const body = (await res.json().catch(() => null)) as
        Envelope<T> | ErrorEnvelope | null;
    if (!res.ok || !body || 'error' in body) {
        const err = body && 'error' in body ? body.error : undefined;
        const retry = Number(res.headers.get('retry-after'));
        throw new ApiError(
            res.status,
            err?.code ?? 'UNEXPECTED_RESPONSE',
            err?.message ?? `Unexpected response (${res.status})`,
            body?.meta?.request_id ?? requestId,
            err?.details ?? [],
            Number.isFinite(retry) && retry > 0 ? retry : undefined,
        );
    }
    return {
        data: body.data,
        requestId: body.meta.request_id,
        nextCursor: body.meta.next_cursor,
        total: body.meta.total,
    };
}
