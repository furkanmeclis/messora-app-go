import { NextResponse } from 'next/server';
import { getAppearance } from '@/lib/server/appearance';

/** The web app manifest, from Settings → PWA (public, like the logo). */
export async function GET() {
    const a = await getAppearance();
    const p = a.pwa;
    if (!p.enabled) return new NextResponse('Not found', { status: 404 });
    const name = p.name || a.organization_name;
    const icon =
        p.icon === 'logo' && a.logo
            ? {
                  src: `/bff/logo?v=${encodeURIComponent(a.logo.version)}`,
                  type: a.logo.content_type,
              }
            : { src: '/bff/favicon?brand=1', type: 'image/svg+xml' };
    const manifest = {
        id: '/',
        name,
        short_name: p.short_name || name.slice(0, 12),
        description: p.description || undefined,
        lang: a.default_locale,
        start_url: p.start_url || '/',
        scope: '/',
        display: p.display,
        display_override: ['window-controls-overlay', p.display],
        theme_color: p.theme_color,
        background_color: p.background_color,
        icons: [
            { ...icon, sizes: 'any', purpose: 'any' },
            { ...icon, sizes: 'any', purpose: 'maskable' },
        ],
        shortcuts: [
            { name: 'Inbox', url: '/inbox' },
            { name: 'Contacts', url: '/contacts' },
            { name: 'Campaigns', url: '/campaigns' },
        ],
    };
    return NextResponse.json(manifest, {
        headers: {
            'Content-Type': 'application/manifest+json',
            'Cache-Control': 'public, max-age=300',
        },
    });
}
