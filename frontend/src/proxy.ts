import { NextResponse, type NextRequest } from 'next/server';
import { requestHost } from '@/lib/host';
import {
    configuredBaseDomain,
    isAdminHost,
    PLATFORM_SEGMENT,
    workspaceImageSource,
} from '@/lib/platform-host';
import { isSignupPage, SDK_CSP } from '@/lib/embedded-signup';

/** Routes of a workspace reachable without a session. */
const PUBLIC = [
    '/login',
    '/invite',
    '/forgot-password',
    '/reset-password',
    '/signup',
    '/verify-email',
    '/confirm-email',
    '/auth/handoff',
    '/logout',
    // The public API reference and the document it renders.
    '/docs/api',
    '/api/health',
    // The organisation logo appears on the sign-in page.
    '/bff/logo',
    '/bff/favicon',
];
const SESSION_COOKIES = ['__Host-messora_session', 'messora_session'];

/** Routes of the platform admin panel reachable without a session. */
const PLATFORM_PUBLIC = ['/login', '/bootstrap', '/logout'];
const PLATFORM_COOKIES = ['__Host-messora_platform', 'messora_platform'];

function matches(pathname: string, list: string[]) {
    return list.some((p) => pathname === p || pathname.startsWith(p + '/'));
}

/**
 * Edge proxy: a per-request CSP nonce (scripts run only with it), a cheap
 * signed-in check that redirects anonymous visitors to the sign-in page,
 * and the admin host. The platform admin panel (`admin.<base>`) is served
 * from an internal route segment that no other host can reach, and the
 * admin host serves nothing else. Authorization itself is always decided
 * by the Go API.
 */
export function proxy(req: NextRequest) {
    const { pathname, search } = req.nextUrl;
    // The browser's host (see requestHost: X-Forwarded-Host first).
    const host = requestHost(req.headers) ?? req.nextUrl.host;
    const base = configuredBaseDomain();
    const admin = isAdminHost(host, base);

    // The internal segment exists only behind the admin host's rewrite.
    if (
        pathname === PLATFORM_SEGMENT ||
        pathname.startsWith(PLATFORM_SEGMENT + '/')
    ) {
        return new NextResponse('Not found', { status: 404 });
    }

    // Served as they are on every host (no platform version needed).
    const shared = pathname === '/api/health' || pathname === '/bff/favicon';
    const isPublic = admin
        ? shared || matches(pathname, PLATFORM_PUBLIC)
        : matches(pathname, PUBLIC);
    const signedIn = (admin ? PLATFORM_COOKIES : SESSION_COOKIES).some((c) =>
        req.cookies.has(c),
    );
    if (!isPublic && !signedIn) {
        const url = req.nextUrl.clone();
        url.pathname = '/login';
        url.search =
            pathname === '/'
                ? ''
                : `?next=${encodeURIComponent(pathname + search)}`;
        return NextResponse.redirect(url);
    }

    const nonce = btoa(crypto.randomUUID());
    const dev = process.env.NODE_ENV === 'development';
    // The realtime WebSocket (Centrifugo behind Caddy's /connection route)
    // lives on this origin; older browsers do not let 'self' cover ws(s):.
    const proto =
        req.headers.get('x-forwarded-proto')?.split(',')[0] ??
        req.nextUrl.protocol;
    const https = proto.trim().replace(/:$/, '') === 'https';
    // Only a plain host[:port] may end up in the policy.
    const plainHost = /^[a-z0-9.\-:[\]]+$/i.test(host);
    const socket = plainHost ? `${https ? 'wss' : 'ws'}://${host}` : '';
    // Other workspaces' logos (the workspace picker).
    const logos = plainHost ? workspaceImageSource(host, https, base) : '';
    // Meta's JavaScript SDK (Embedded Signup) runs only on its settings
    // page: its script, frames and calls are allowed there and nowhere else.
    const meta = isSignupPage(pathname);
    const csp = [
        `default-src 'self'`,
        `script-src 'self' 'nonce-${nonce}' 'strict-dynamic'${meta ? ' ' + SDK_CSP.script : ''}${dev ? " 'unsafe-eval'" : ''}`,
        `style-src 'self' 'unsafe-inline'`,
        `img-src 'self' data: blob:${logos ? ' ' + logos : ''}${meta ? ' ' + SDK_CSP.img : ''}`,
        `font-src 'self'`,
        `connect-src 'self'${socket ? ' ' + socket : ''}${meta ? ' ' + SDK_CSP.connect : ''}${dev ? ' ws: wss:' : ''}`,
        `frame-src 'self'${meta ? ' ' + SDK_CSP.frame : ''}`,
        `object-src 'none'`,
        `base-uri 'self'`,
        `form-action 'self'`,
        // Nothing frames the panel, not even its own pages.
        `frame-ancestors 'none'`,
    ].join('; ');

    const requestHeaders = new Headers(req.headers);
    requestHeaders.set('x-nonce', nonce);
    requestHeaders.set('x-pathname', pathname);
    requestHeaders.set('Content-Security-Policy', csp);
    let res: NextResponse;
    if (admin && !shared) {
        const url = req.nextUrl.clone();
        url.pathname = PLATFORM_SEGMENT + (pathname === '/' ? '' : pathname);
        res = NextResponse.rewrite(url, {
            request: { headers: requestHeaders },
        });
    } else {
        res = NextResponse.next({ request: { headers: requestHeaders } });
    }
    res.headers.set('Content-Security-Policy', csp);
    return res;
}

export const config = {
    matcher: [
        // Everything except static assets and the upload routes, which
        // stream files of up to 100 MB (a proxy would buffer the body and
        // cut it at 10 MB); they check the session themselves.
        '/((?!_next/static|_next/image|favicon.ico|robots.txt|bff/contact-imports|bff/media-upload|bff/template-sample|sw\\.js).*)',
    ],
};
