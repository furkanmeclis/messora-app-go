import { NextResponse, type NextRequest } from 'next/server';
import { streamDownload, streamUpload, UUID_RE } from '@/lib/server/stream';

/** Downloads a company's official invoice PDF (admin host). */
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
        `/platform/billing/invoices/${id}/pdf`,
        'platform',
    );
}

/** Same limit as the API (x-messora-raw-body of uploadPlatformInvoicePdf). */
const MAX_BYTES = 5 * 1024 * 1024;

/** Attaches the official invoice PDF (the raw body) with the platform session. */
export async function PUT(
    req: NextRequest,
    ctx: { params: Promise<{ id: string }> },
) {
    const { id } = await ctx.params;
    if (!UUID_RE.test(id)) {
        return new NextResponse('Not found', { status: 404 });
    }
    return streamUpload(
        req,
        `/platform/billing/invoices/${id}/pdf`,
        MAX_BYTES,
        'PUT',
        'platform',
    );
}
