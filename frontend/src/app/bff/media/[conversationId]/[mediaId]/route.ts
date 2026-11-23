import { NextResponse, type NextRequest } from 'next/server';
import { serverConfig } from '@/lib/server/config';
import { getSessionToken } from '@/lib/server/session';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

/** Headers of the API's answer that the browser may see. */
const PASS = [
    'content-type',
    'content-length',
    'content-disposition',
    'content-range',
    'accept-ranges',
    'last-modified',
    'cache-control',
];

/**
 * Streams a stored media file of a conversation from the Go API with the
 * user's session. The API decides visibility and sets a safe Content-Type
 * and Content-Disposition; this route adds nosniff and a sandboxing policy
 * again so the browser can never treat a file as part of the panel.
 */
export async function GET(
    req: NextRequest,
    ctx: { params: Promise<{ conversationId: string; mediaId: string }> },
) {
    const { conversationId, mediaId } = await ctx.params;
    if (!UUID.test(conversationId) || !UUID.test(mediaId)) {
        return new NextResponse('Not found', { status: 404 });
    }
    // Only the panel itself may load files (no cross-site embedding).
    const site = req.headers.get('sec-fetch-site');
    if (site && site !== 'same-origin' && site !== 'none') {
        return new NextResponse('Forbidden', { status: 403 });
    }
    const token = await getSessionToken();
    if (!token) return new NextResponse('Unauthorized', { status: 401 });

    const { apiInternalUrl, bffSharedSecret } = serverConfig();
    const h = new Headers({
        'X-Messora-BFF': bffSharedSecret,
        'X-Request-ID': crypto.randomUUID(),
        Authorization: `Bearer ${token}`,
    });
    const range = req.headers.get('range');
    if (range) h.set('Range', range);
    let res: Response;
    try {
        res = await fetch(
            `${apiInternalUrl}/api/v1/conversations/${conversationId}/media/${mediaId}`,
            { headers: h, cache: 'no-store' },
        );
    } catch {
        return new NextResponse('Unavailable', { status: 503 });
    }
    if (!res.ok || !res.body) {
        const status = [401, 403, 404, 409].includes(res.status)
            ? res.status
            : 502;
        return new NextResponse(
            status === 409 ? 'Not downloaded yet' : 'Not found',
            { status, headers: { 'X-Content-Type-Options': 'nosniff' } },
        );
    }
    const out = new Headers();
    for (const k of PASS) {
        const v = res.headers.get(k);
        if (v) out.set(k, v);
    }
    out.set('X-Content-Type-Options', 'nosniff');
    out.set('Content-Security-Policy', "default-src 'none'; sandbox");
    return new NextResponse(res.body, { status: res.status, headers: out });
}
