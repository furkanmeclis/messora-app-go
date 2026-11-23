import type { Metadata } from 'next';
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

export async function generateMetadata(): Promise<Metadata> {
    const { organization_name: org } = await getAppearance();
    return {
        title: { default: org, template: `%s · ${org}` },
        description: 'Self-hosted WhatsApp Business Cloud API platform',
        robots: { index: false, follow: false },
    };
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
                {/* Pages inside a desktop-mode window hide the panel chrome. */}
                <script
                    nonce={nonce}
                    dangerouslySetInnerHTML={{
                        __html: "if(window.top!==window.self)document.documentElement.setAttribute('data-embedded','')",
                    }}
                />
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
                        <TooltipProvider>{children}</TooltipProvider>
                        <Toaster richColors position="bottom-right" />
                    </I18nProvider>
                </ThemeProvider>
            </body>
        </html>
    );
}
