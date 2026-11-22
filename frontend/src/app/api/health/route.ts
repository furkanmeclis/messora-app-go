// Liveness probe for the container healthcheck. It deliberately does not
// call the Go API: the web container is healthy when it can serve requests.
export const dynamic = 'force-dynamic';

export function GET() {
    return Response.json(
        { status: 'ok' },
        { headers: { 'Cache-Control': 'no-store' } },
    );
}
