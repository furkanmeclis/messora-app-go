import { NextResponse, type NextRequest } from 'next/server';
import { streamDownload, UUID_RE } from '@/lib/server/stream';

/** Downloads a company's proforma as PDF (admin host). */
export async function GET(
    req: NextRequest,
    ctx: { params: Promise<{ id: string }> },
) {
    const { id } = await ctx.params;
    if (!UUID_RE.test(id)) {
        return new NextResponse('Not found', { status: 404 });
    }
    return streamDownload(
        req,
        `/platform/billing/proformas/${id}/pdf`,
        'platform',
    );
}
