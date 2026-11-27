import type { Metadata } from 'next';
import { ServiceWorkerCleanup } from '@/components/service-worker-cleanup';
import { headers } from 'next/headers';
import { ThemeProvider } from '@/components/theme-provider';
import { Toaster } from '@/components/ui/sonner';
import { TooltipProvider } from '@/components/ui/tooltip';
import { I18nProvider } from '@/i18n/client';
import { getLocale } from '@/i18n/server';
import { getAppearance } from '@/lib/server/appearance';
import './globals.css';

// Every page is rendered per request: pages depend on the session, and the
// CSP nonce changes on each response.
export const dynamic = 'force-dynamic';

export async function generateMetadata(): Promise<Metadata> {
    const { organization_name: org, logo } = await getAppearance();
    // The version changes with the logo, so browsers refetch it.
    const v = encodeURIComponent(logo?.version ?? 'messora');
    return {
        title: { default: org, template: `%s · ${org}` },
        icons: {
            icon: [
                {
                    url: `/bff/favicon?v=${v}`,
                    type: logo?.content_type ?? 'image/svg+xml',
                },
            ],
        },
        description: 'WhatsApp Business Cloud API platform',
        robots: { index: false, follow: false },
    };
}

export default async function RootLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    const [nonce, locale] = await Promise.all([
        headers().then((h) => h.get('x-nonce') ?? undefined),
        getLocale(),
    ]);
    return (
        <html lang={locale} className="font-sans" suppressHydrationWarning>
            <body className="antialiased">
                <ThemeProvider nonce={nonce}>
                    <I18nProvider locale={locale}>
                        <ServiceWorkerCleanup />
                        <TooltipProvider>{children}</TooltipProvider>
                        <Toaster richColors position="bottom-right" />
                    </I18nProvider>
                </ThemeProvider>
            </body>
        </html>
    );
}
