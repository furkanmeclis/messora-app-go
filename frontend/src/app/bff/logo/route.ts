import { NextResponse, type NextRequest } from 'next/server';
import { serverConfig } from '@/lib/server/config';
import { streamUpload } from '@/lib/server/stream';

/**
 * The organisation logo. Public (the sign-in page shows it); the API sets
 * the image type and a sandboxing policy, which pass through unchanged.
 */
export async function GET() {
    const { apiInternalUrl, bffSharedSecret } = serverConfig();
    let res: Response;
    try {
        res = await fetch(`${apiInternalUrl}/api/v1/appearance/logo`, {
            headers: {
                'X-Messora-BFF': bffSharedSecret,
                'X-Request-ID': crypto.randomUUID(),
            },
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
            'Cache-Control': 'public, max-age=300',
            'X-Content-Type-Options': 'nosniff',
            'Content-Security-Policy':
                "default-src 'none'; style-src 'unsafe-inline'; sandbox",
        },
    });
}

/** Replaces the logo (2 MB at most, checked again by the API). */
export async function PUT(req: NextRequest) {
    return streamUpload(req, '/settings/logo', 2 * 1024 * 1024, 'PUT');
}
