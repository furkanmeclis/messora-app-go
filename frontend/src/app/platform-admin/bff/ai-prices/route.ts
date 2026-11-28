import type { NextRequest } from 'next/server';
import { streamUpload } from '@/lib/server/stream';

/** Same limit as the API (x-messora-raw-body of importAIPrices). */
const MAX_BYTES = 16 * 1024 * 1024;

/**
 * Imports LiteLLM's price list (admin host only): the multipart body goes
 * to the API unchanged with the platform session.
 */
export async function POST(req: NextRequest) {
    return streamUpload(
        req,
        '/platform/ai-prices/import',
        MAX_BYTES,
        'POST',
        'platform',
    );
}
