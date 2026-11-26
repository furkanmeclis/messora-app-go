import { NextResponse, type NextRequest } from 'next/server';
import { streamDownload, UUID_RE } from '@/lib/server/stream';

const STATUS_RE = /^[a-z_,]{0,500}$/;

/**
 * Downloads a campaign's recipients matching the table's filters as CSV or
 * Excel (up to 20,000 rows; larger selections run as background exports).
 * Only known filters are passed on.
 */
export async function GET(
    req: NextRequest,
    ctx: { params: Promise<{ campaignId: string }> },
) {
    const { campaignId } = await ctx.params;
    if (!UUID_RE.test(campaignId)) {
        return new NextResponse('Not found', { status: 404 });
    }
    const sp = req.nextUrl.searchParams;
    const q = new URLSearchParams();
    q.set('format', sp.get('format') === 'xlsx' ? 'xlsx' : 'csv');
    const text = (sp.get('q') ?? '').trim().slice(0, 100);
    if (text) q.set('q', text);
    const status = sp.get('status') ?? '';
    if (status && STATUS_RE.test(status)) q.set('status', status);
    for (const key of ['tag_id', 'group_id', 'segment_id']) {
        const v = sp.get(key);
        if (v && UUID_RE.test(v)) q.set(key, v);
    }
    const ct = sp.get('contact_type');
    if (ct !== null) q.set('contact_type', ct.slice(0, 60));
    const key = (sp.get('attr_key') ?? '').slice(0, 100);
    if (key) {
        q.set('attr_key', key);
        q.set('attr_value', (sp.get('attr_value') ?? '').slice(0, 200));
    }
    return streamDownload(
        req,
        `/campaigns/${campaignId}/recipients/export?${q}`,
    );
}
