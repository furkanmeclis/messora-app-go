import type { NextRequest } from 'next/server';
import { streamUpload } from '@/lib/server/stream';

/** Same limit as the API (documents; smaller types are checked there). */
const MAX_BYTES = 100 * 1024 * 1024;

/**
 * Uploads a file to send in a conversation. The raw body streams to the API,
 * which forwards it to Meta; the proxy skips this route (see src/proxy.ts)
 * and the session is checked here.
 */
export async function POST(req: NextRequest) {
    const p = req.nextUrl.searchParams;
    const q = new URLSearchParams({
        filename: (p.get('filename') ?? '').trim().slice(0, 240) || 'file',
    });
    const number = p.get('phone_number_id');
    if (number && /^[0-9a-f-]{36}$/i.test(number)) {
        q.set('phone_number_id', number);
    }
    return streamUpload(req, `/media?${q}`, MAX_BYTES);
}
