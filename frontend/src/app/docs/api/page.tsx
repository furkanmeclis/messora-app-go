import type { Metadata } from 'next';
import { getLocale, getT } from '@/i18n/server';
import { ApiReference } from './api-reference';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('apiDocs.title') };
}

/**
 * Public API reference (no sign-in): Scalar renders the public OpenAPI
 * document the Go API serves at /api/v1/openapi.json. Scalar is bundled
 * from npm and served from this origin; no CDN, fonts or telemetry.
 */
export default async function ApiDocsPage() {
    const locale = await getLocale();
    return (
        <main className="min-h-svh bg-background">
            <ApiReference locale={locale} />
        </main>
    );
}
