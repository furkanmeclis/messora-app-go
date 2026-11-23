import { NextResponse } from 'next/server';
import { getAppearance } from '@/lib/server/appearance';
import { parseColour, toHex } from '@/theme/colour';
import { resolvePalette } from '@/theme/css';

/**
 * The browser tab icon: the organisation logo when one is uploaded,
 * otherwise Messora's mark in the theme's primary colour.
 */
export async function GET() {
    const a = await getAppearance();
    if (a.logo) {
        return new NextResponse(null, {
            status: 307,
            headers: {
                Location: `/bff/logo?v=${encodeURIComponent(a.logo.version)}`,
            },
        });
    }
    const pal = resolvePalette(a.theme, 'light');
    const bg = toHex({
        ...(parseColour(pal.primary) ?? { r: 0.3, g: 0.3, b: 0.9, a: 1 }),
        a: 1,
    });
    const fg = toHex({
        ...(parseColour(pal['primary-foreground']) ?? {
            r: 1,
            g: 1,
            b: 1,
            a: 1,
        }),
        a: 1,
    });
    const svg = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
<rect width="32" height="32" rx="8" fill="${bg}"/>
<g transform="translate(4 4)" fill="none" stroke="${fg}" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
<path d="M14 9a2 2 0 0 1-2 2H6l-4 4V4a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2z"/>
<path d="M18 9h2a2 2 0 0 1 2 2v11l-4-4h-6a2 2 0 0 1-2-2v-1"/>
</g></svg>`;
    return new NextResponse(svg, {
        headers: {
            'Content-Type': 'image/svg+xml',
            'Cache-Control': 'public, max-age=300',
            'X-Content-Type-Options': 'nosniff',
            'Content-Security-Policy':
                "default-src 'none'; style-src 'unsafe-inline'",
        },
    });
}
