import { NextResponse, type NextRequest } from 'next/server';
import { serverConfig } from '@/lib/server/config';
import { streamUpload } from '@/lib/server/stream';
import { invalidateAppearance } from '@/lib/server/appearance';
import { forwardHost } from '@/lib/server/tenant-host';

/**
 * The organisation logo. Public (the sign-in page shows it); the API sets
 * the image type and a sandboxing policy, which pass through unchanged.
 */
export async function GET(req: NextRequest) {
    const { apiInternalUrl, bffSharedSecret } = serverConfig();
    const h = new Headers({
        'X-Messora-BFF': bffSharedSecret,
        'X-Request-ID': crypto.randomUUID(),
    });
    forwardHost(req.headers, h);
    let res: Response;
    try {
        res = await fetch(`${apiInternalUrl}/api/v1/appearance/logo`, {
            headers: h,
            cache: 'no-store',
        });
    } catch {
        return new NextResponse('Unavailable', { status: 503 });
    }
    if (!res.ok || !res.body) {
        return new NextResponse('Not found', {
            status: res.status === 404 ? 404 : 502,
        });
    }
    const type = res.headers.get('content-type') ?? '';
    if (!/^image\/(png|jpeg|webp|svg\+xml)$/.test(type)) {
        return new NextResponse('Not found', { status: 404 });
    }
    return new NextResponse(res.body, {
        headers: {
            'Content-Type': type,
            // Each company's address has its own logo.
            'Cache-Control': 'public, max-age=300',
            Vary: 'Host',
            'X-Content-Type-Options': 'nosniff',
            'Content-Security-Policy':
                "default-src 'none'; style-src 'unsafe-inline'; sandbox",
        },
    });
}

/** Replaces the logo (2 MB at most, checked again by the API). */
export async function PUT(req: NextRequest) {
    const res = await streamUpload(
        req,
        '/settings/logo',
        2 * 1024 * 1024,
        'PUT',
    );
    invalidateAppearance();
    return res;
}
