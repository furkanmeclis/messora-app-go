import type { NextRequest } from 'next/server';
import { streamDownload, UUID_RE } from '@/lib/server/stream';

/**
 * Downloads the contacts matching the list filters as CSV or Excel. The API
 * checks `contacts.export` and streams the rows; only known filters are
 * passed on.
 */
export async function GET(req: NextRequest) {
    const sp = req.nextUrl.searchParams;
    const q = new URLSearchParams();
    q.set('format', sp.get('format') === 'xlsx' ? 'xlsx' : 'csv');
    const text = (sp.get('q') ?? '').trim().slice(0, 100);
    if (text) q.set('q', text);
    for (const key of ['tag_id', 'group_id', 'segment_id']) {
        const v = sp.get(key);
        if (v && UUID_RE.test(v)) q.set(key, v);
    }
    return streamDownload(req, `/contact-exports?${q}`);
}
