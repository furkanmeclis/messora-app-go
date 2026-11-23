import 'server-only';

import { NextResponse, type NextRequest } from 'next/server';
import { serverConfig } from './config';
import { getSessionToken } from './session';

/** Headers of a file download that the browser may see. */
const PASS = [
    'content-type',
    'content-length',
    'content-disposition',
    'last-modified',
    'cache-control',
];

/**
 * Only the panel itself may call these routes: a cross-site page must not
 * trigger an upload or a download with the user's cookie.
 */
function crossSite(req: NextRequest): boolean {
    const site = req.headers.get('sec-fetch-site');
    return !!site && site !== 'same-origin' && site !== 'none';
}

async function apiHeaders(req: NextRequest): Promise<Headers | null> {
    const token = await getSessionToken();
    if (!token) return null;
    const { bffSharedSecret } = serverConfig();
    const h = new Headers({
        'X-Messora-BFF': bffSharedSecret,
        'X-Request-ID': crypto.randomUUID(),
        Authorization: `Bearer ${token}`,
    });
    const xff = req.headers.get('x-forwarded-for');
    if (xff) h.set('X-Forwarded-For', xff.split(',').pop()!.trim());
    const ua = req.headers.get('user-agent');
    if (ua) h.set('User-Agent', ua.slice(0, 512));
    return h;
}

function plain(status: number, text: string) {
    return new NextResponse(text, {
        status,
        headers: {
            'Content-Type': 'text/plain; charset=utf-8',
            'X-Content-Type-Options': 'nosniff',
        },
    });
}

/** The API's error message, for a download that failed. */
async function errorText(res: Response): Promise<string> {
    const body = (await res.json().catch(() => null)) as {
        error?: { message?: string };
    } | null;
    return body?.error?.message ?? `The download failed (${res.status}).`;
}

/**
 * Streams a file from the Go API (`path` below /api/v1) to the browser with
 * the user's session. Nothing is buffered, so large exports start at once.
 */
export async function streamDownload(
    req: NextRequest,
    path: string,
): Promise<NextResponse> {
    if (crossSite(req)) return plain(403, 'Forbidden');
    const h = await apiHeaders(req);
    if (!h) return plain(401, 'Your session has ended. Sign in again.');
    let res: Response;
    try {
        res = await fetch(`${serverConfig().apiInternalUrl}/api/v1${path}`, {
            headers: h,
            cache: 'no-store',
            signal: req.signal,
        });
    } catch {
        return plain(503, 'The Messora API is not reachable.');
    }
    if (!res.ok || !res.body) {
        const status = [400, 401, 403, 404, 422].includes(res.status)
            ? res.status
            : 502;
        return plain(status, await errorText(res));
    }
    const out = new Headers();
    for (const k of PASS) {
        const v = res.headers.get(k);
        if (v) out.set(k, v);
    }
    out.set('Cache-Control', 'no-store');
    out.set('X-Content-Type-Options', 'nosniff');
    out.set('Content-Security-Policy', "default-src 'none'; sandbox");
    return new NextResponse(res.body, { status: 200, headers: out });
}

/**
 * Streams the request body (a file) to the Go API and returns its JSON
 * answer unchanged, so the browser sees the usual envelope and error
 * details. The body is never held in memory.
 */
export async function streamUpload(
    req: NextRequest,
    path: string,
    maxBytes: number,
    method: 'POST' | 'PUT' = 'POST',
): Promise<NextResponse> {
    const fail = (status: number, code: string, message: string) =>
        NextResponse.json(
            { error: { code, message, details: [] }, meta: {} },
            { status },
        );
    if (crossSite(req)) return fail(403, 'FORBIDDEN', 'Forbidden');
    const h = await apiHeaders(req);
    if (!h) {
        return fail(401, 'UNAUTHORIZED', 'Your session has ended.');
    }
    const length = Number(req.headers.get('content-length') ?? '0');
    if (length > maxBytes) {
        return fail(
            413,
            'VALIDATION_ERROR',
            `The file is larger than ${Math.round(maxBytes / 1024 / 1024)} MB.`,
        );
    }
    if (!req.body) {
        return fail(422, 'VALIDATION_ERROR', 'The file is empty.');
    }
    h.set(
        'Content-Type',
        req.headers.get('content-type') || 'application/octet-stream',
    );
    if (length > 0) h.set('Content-Length', String(length));
    h.set('Accept', 'application/json');
    let res: Response;
    try {
        res = await fetch(`${serverConfig().apiInternalUrl}/api/v1${path}`, {
            method,
            headers: h,
            body: req.body,
            cache: 'no-store',
            signal: req.signal,
            // Required by Node's fetch to send a streamed body.
            duplex: 'half',
        } as RequestInit & { duplex: 'half' });
    } catch {
        return fail(
            503,
            'API_UNREACHABLE',
            'The Messora API is not reachable. Try again in a moment.',
        );
    }
    const text = await res.text();
    return new NextResponse(text, {
        status: res.status,
        headers: {
            'Content-Type': 'application/json',
            'Cache-Control': 'no-store',
            'X-Content-Type-Options': 'nosniff',
        },
    });
}

export const UUID_RE =
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;
