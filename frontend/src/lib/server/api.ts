import 'server-only';

import { serverConfig } from './config';

/** Error body returned by the Messora API. */
export type ApiErrorBody = {
    code: string;
    message: string;
    details: { field?: string; code: string; message?: string }[];
};

export class ApiError extends Error {
    constructor(
        readonly status: number,
        readonly code: string,
        message: string,
        readonly requestId: string,
        readonly details: ApiErrorBody['details'] = [],
    ) {
        super(message);
        this.name = 'ApiError';
    }
}

type Envelope<T> = {
    data: T;
    meta: { request_id: string; next_cursor?: string };
};
type ErrorEnvelope = { error: ApiErrorBody; meta: { request_id: string } };

/**
 * Calls the Go API over the internal network. Every call carries the BFF
 * secret (so the API accepts it as coming from the panel) and a request ID
 * that appears in both services' logs.
 */
export async function apiFetch<T>(
    path: string,
    init: RequestInit & { timeoutMs?: number } = {},
): Promise<{ data: T; requestId: string }> {
    const { apiInternalUrl, bffSharedSecret } = serverConfig();
    const requestId = crypto.randomUUID();
    const headers = new Headers(init.headers);
    headers.set('Accept', 'application/json');
    headers.set('X-Messora-BFF', bffSharedSecret);
    headers.set('X-Request-ID', requestId);

    let res: Response;
    try {
        res = await fetch(`${apiInternalUrl}${path}`, {
            ...init,
            headers,
            cache: 'no-store',
            signal: AbortSignal.timeout(init.timeoutMs ?? 10_000),
        });
    } catch {
        throw new ApiError(
            503,
            'API_UNREACHABLE',
            'The Messora API is not reachable',
            requestId,
        );
    }

    const body = (await res.json().catch(() => null)) as
        Envelope<T> | ErrorEnvelope | null;
    if (!res.ok || !body || 'error' in body) {
        const err = body && 'error' in body ? body.error : undefined;
        throw new ApiError(
            res.status,
            err?.code ?? 'UNEXPECTED_RESPONSE',
            err?.message ?? `Unexpected response (${res.status})`,
            body?.meta?.request_id ?? requestId,
            err?.details,
        );
    }
    return { data: body.data, requestId: body.meta.request_id };
}
