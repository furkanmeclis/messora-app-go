import { NextResponse, type NextRequest } from 'next/server';

/** Routes reachable without a session. */
const PUBLIC = [
    '/login',
    '/setup',
    '/invite',
    '/forgot-password',
    '/reset-password',
    '/logout',
    '/api/health',
];
const SESSION_COOKIES = ['__Host-messora_session', 'messora_session'];

/**
 * Edge proxy: a per-request CSP nonce (scripts run only with it) and a cheap
 * signed-in check that redirects anonymous visitors to the sign-in page.
 * Authorization itself is always decided by the Go API.
 */
export function proxy(req: NextRequest) {
    const { pathname, search } = req.nextUrl;
    const isPublic = PUBLIC.some(
        (p) => pathname === p || pathname.startsWith(p + '/'),
    );
    const signedIn = SESSION_COOKIES.some((c) => req.cookies.has(c));
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
    const host = req.headers.get('host') ?? req.nextUrl.host;
    const proto =
        req.headers.get('x-forwarded-proto')?.split(',')[0] ??
        req.nextUrl.protocol;
    const https = proto.trim().replace(/:$/, '') === 'https';
    // Only a plain host[:port] may end up in the policy.
    const socket = /^[a-z0-9.\-:[\]]+$/i.test(host)
        ? `${https ? 'wss' : 'ws'}://${host}`
        : '';
    const csp = [
        `default-src 'self'`,
        `script-src 'self' 'nonce-${nonce}' 'strict-dynamic'${dev ? " 'unsafe-eval'" : ''}`,
        `style-src 'self' 'unsafe-inline'`,
        `img-src 'self' data: blob:`,
        `font-src 'self'`,
        `connect-src 'self'${socket ? ' ' + socket : ''}${dev ? ' ws: wss:' : ''}`,
        `object-src 'none'`,
        `base-uri 'self'`,
        `form-action 'self'`,
        `frame-ancestors 'none'`,
    ].join('; ');

    const requestHeaders = new Headers(req.headers);
    requestHeaders.set('x-nonce', nonce);
    requestHeaders.set('x-pathname', pathname);
    requestHeaders.set('Content-Security-Policy', csp);
    const res = NextResponse.next({ request: { headers: requestHeaders } });
    res.headers.set('Content-Security-Policy', csp);
    return res;
}

export const config = {
    matcher: [
        // Everything except static assets.
        '/((?!_next/static|_next/image|favicon.ico|robots.txt).*)',
    ],
};
