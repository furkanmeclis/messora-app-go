import type { Metadata, Viewport } from 'next';
import { PwaProvider } from '@/components/pwa';
import { headers } from 'next/headers';
import { ThemeProvider } from '@/components/theme-provider';
import { Toaster } from '@/components/ui/sonner';
import { TooltipProvider } from '@/components/ui/tooltip';
import { I18nProvider } from '@/i18n/client';
import { getLocale } from '@/i18n/server';
import { getAppearance } from '@/lib/server/appearance';
import { themeCss } from '@/theme/css';
import './globals.css';

// Every page is rendered per request: pages depend on the session, and the
// CSP nonce changes on each response.
export const dynamic = 'force-dynamic';

/** A short, stable fingerprint (cache key, not security). */
function hash(text: string): string {
    let h = 5381;
    for (let i = 0; i < text.length; i++)
        h = ((h << 5) + h + text.charCodeAt(i)) | 0;
    return (h >>> 0).toString(36);
}

export async function generateMetadata(): Promise<Metadata> {
    const { organization_name: org, logo, theme, pwa } = await getAppearance();
    // The version changes with the logo or theme, so browsers refetch it.
    const v = encodeURIComponent(logo?.version ?? hash(JSON.stringify(theme)));
    return {
        title: { default: org, template: `%s · ${org}` },
        ...(pwa.enabled
            ? {
                  manifest: '/bff/manifest',
                  appleWebApp: {
                      capable: true,
                      title: pwa.short_name || pwa.name || org,
                      statusBarStyle: 'default' as const,
                  },
              }
            : {}),
        icons: {
            icon: [
                {
                    url: `/bff/favicon?v=${v}`,
                    type: logo?.content_type ?? 'image/svg+xml',
                },
            ],
        },
        description: 'Self-hosted WhatsApp Business Cloud API platform',
        robots: { index: false, follow: false },
    };
}

export async function generateViewport(): Promise<Viewport> {
    const { pwa } = await getAppearance();
    return pwa.enabled ? { themeColor: pwa.theme_color } : {};
}

export default async function RootLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    const [nonce, appearance, locale] = await Promise.all([
        headers().then((h) => h.get('x-nonce') ?? undefined),
        getAppearance(),
        getLocale(),
    ]);
    return (
        <html lang={locale} className="font-sans" suppressHydrationWarning>
            <head>
                {/* The workspace theme, before first paint (no flash). */}
                <style
                    id="messora-theme"
                    nonce={nonce}
                    dangerouslySetInnerHTML={{
                        __html: themeCss(appearance.theme),
                    }}
                />
            </head>
            <body className="antialiased">
                <ThemeProvider
                    nonce={nonce}
                    defaultTheme={appearance.theme.mode}
                >
                    <I18nProvider locale={locale}>
                        <PwaProvider enabled={appearance.pwa.enabled}>
                            <TooltipProvider>{children}</TooltipProvider>
                            <Toaster richColors position="bottom-right" />
                        </PwaProvider>
                    </I18nProvider>
                </ThemeProvider>
            </body>
        </html>
    );
}
