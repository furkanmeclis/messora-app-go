import { NextResponse, type NextRequest } from 'next/server';
import { streamDownload, UUID_RE } from '@/lib/server/stream';

/** Downloads a finished background export of a campaign. */
export async function GET(
    req: NextRequest,
    ctx: { params: Promise<{ campaignId: string; exportId: string }> },
) {
    const { campaignId, exportId } = await ctx.params;
    if (!UUID_RE.test(campaignId) || !UUID_RE.test(exportId)) {
        return new NextResponse('Not found', { status: 404 });
    }
    return streamDownload(
        req,
        `/campaigns/${campaignId}/exports/${exportId}/download`,
    );
}
