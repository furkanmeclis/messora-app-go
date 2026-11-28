import type { NextRequest } from 'next/server';
import { streamUpload } from '@/lib/server/stream';

/** Same limit as the API (x-messora-raw-body of importPlatformTransfers). */
const MAX_BYTES = 2 * 1024 * 1024;

/** Imports a bank statement CSV into the payments inbox (admin host). */
export async function POST(req: NextRequest) {
    return streamUpload(
        req,
        '/platform/billing/transfers/import',
        MAX_BYTES,
        'POST',
        'platform',
    );
}
