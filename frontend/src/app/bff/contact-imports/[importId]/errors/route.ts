import { NextResponse, type NextRequest } from 'next/server';
import { streamDownload, UUID_RE } from '@/lib/server/stream';

/** The rows of an import that failed, with the reason, as CSV. */
export async function GET(
    req: NextRequest,
    ctx: { params: Promise<{ importId: string }> },
) {
    const { importId } = await ctx.params;
    if (!UUID_RE.test(importId)) {
        return new NextResponse('Not found', { status: 404 });
    }
    return streamDownload(req, `/contact-imports/${importId}/errors`);
}
