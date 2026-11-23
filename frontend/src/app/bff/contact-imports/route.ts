import type { NextRequest } from 'next/server';
import { streamUpload } from '@/lib/server/stream';

/** Same limit as the API (MaxImportBytes). */
const MAX_BYTES = 50 * 1024 * 1024;

/**
 * Uploads an import file: the raw body goes to the API as it arrives. The
 * proxy does not run for this route (see src/proxy.ts), so the file is never
 * buffered in memory; the session is checked here instead.
 */
export async function POST(req: NextRequest) {
    const filename = (req.nextUrl.searchParams.get('filename') ?? '')
        .trim()
        .slice(0, 255);
    const q = new URLSearchParams({ filename: filename || 'contacts.csv' });
    return streamUpload(req, `/contact-imports?${q}`, MAX_BYTES);
}
