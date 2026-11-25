import type { NextRequest } from 'next/server';
import { streamUpload } from '@/lib/server/stream';

/** Same limit as the API (x-messora-raw-body of importAIPrices). */
const MAX_BYTES = 16 * 1024 * 1024;

/**
 * Imports LiteLLM's price list: the multipart body (the JSON in "file",
 * optionally "effective_from") goes to the API unchanged.
 */
export async function POST(req: NextRequest) {
    return streamUpload(req, '/ai/prices/import', MAX_BYTES);
}
