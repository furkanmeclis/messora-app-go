import type { NextRequest } from 'next/server';
import { streamUpload } from '@/lib/server/stream';

/** Documents may be up to 100 MB; the API checks each type's limit. */
const MAX_BYTES = 100 * 1024 * 1024;
const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/**
 * Uploads the example file of a media-header template; the API sends it to
 * Meta and returns the handle. The proxy skips this route (src/proxy.ts),
 * so the session is checked by streamUpload.
 */
export async function POST(req: NextRequest) {
    const p = req.nextUrl.searchParams;
    const account = p.get('account_id') ?? '';
    if (!UUID.test(account)) {
        return Response.json(
            {
                error: {
                    code: 'VALIDATION_ERROR',
                    message: 'Choose an account.',
                },
            },
            { status: 422 },
        );
    }
    const q = new URLSearchParams({
        account_id: account,
        filename: (p.get('filename') ?? '').trim().slice(0, 240) || 'example',
    });
    return streamUpload(req, `/templates/samples?${q}`, MAX_BYTES);
}
