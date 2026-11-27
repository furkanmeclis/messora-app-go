import type { NextRequest } from 'next/server';
import { streamUpload } from '@/lib/server/stream';

/** Same limit as the API (x-messora-raw-body of importMetaRates). */
const MAX_BYTES = 2 * 1024 * 1024;

/**
 * Imports Meta's rate card (admin host only): the multipart body (a CSV in
 * the "file" field) goes to the API unchanged with the platform session.
 */
export async function POST(req: NextRequest) {
    return streamUpload(
        req,
        '/platform/meta-rates/import',
        MAX_BYTES,
        'POST',
        'platform',
    );
}
