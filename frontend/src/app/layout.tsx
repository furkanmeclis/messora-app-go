import type { Metadata } from 'next';
import { Geist, Geist_Mono } from 'next/font/google';
import { headers } from 'next/headers';
import { ThemeProvider } from '@/components/theme-provider';
import { Toaster } from '@/components/ui/sonner';
import { TooltipProvider } from '@/components/ui/tooltip';
import { cn } from '@/lib/utils';
import './globals.css';

// Self-hosted at build time: the browser never contacts Google.
const sans = Geist({
    subsets: ['latin', 'latin-ext'],
    variable: '--font-sans',
});
const mono = Geist_Mono({ subsets: ['latin'], variable: '--font-mono' });

// Every page is rendered per request: pages depend on the session, and the
// CSP nonce changes on each response.
export const dynamic = 'force-dynamic';

export const metadata: Metadata = {
    title: { default: 'Messora', template: '%s · Messora' },
    description: 'Self-hosted WhatsApp Business Cloud API platform',
    robots: { index: false, follow: false },
};

export default async function RootLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    const nonce = (await headers()).get('x-nonce') ?? undefined;
    return (
        <html
            lang="en"
            className={cn(sans.variable, mono.variable, 'font-sans')}
            suppressHydrationWarning
        >
            <body className="antialiased">
                <ThemeProvider nonce={nonce}>
                    <TooltipProvider>{children}</TooltipProvider>
                    <Toaster richColors position="bottom-right" />
                </ThemeProvider>
            </body>
        </html>
    );
}
