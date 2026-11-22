import type { Metadata } from 'next';
import { Instrument_Sans } from 'next/font/google';
import { cookies } from 'next/headers';
import { Toaster } from '@/components/ui/sonner';
import './globals.css';

// Self-hosted at build time: the browser never contacts Google.
const instrumentSans = Instrument_Sans({
    subsets: ['latin', 'latin-ext'],
    variable: '--font-instrument-sans',
});

export const metadata: Metadata = {
    title: { default: 'Messora', template: '%s · Messora' },
    description: 'Self-hosted WhatsApp Business Cloud API platform',
    robots: { index: false, follow: false },
};

// Applies the stored theme before first paint (ported from ZeroMsg's root view).
const themeScript = `(function(){try{var a=localStorage.getItem('appearance')||'system';var d=a==='dark'||(a==='system'&&matchMedia('(prefers-color-scheme: dark)').matches);var e=document.documentElement;e.classList.toggle('dark',d);e.style.colorScheme=d?'dark':'light';}catch(_){}})();`;

export default async function RootLayout({
    children,
}: {
    children: React.ReactNode;
}) {
    const appearance = (await cookies()).get('appearance')?.value;
    return (
        <html
            lang="en"
            className={`${instrumentSans.variable} ${appearance === 'dark' ? 'dark' : ''}`}
            suppressHydrationWarning
        >
            <head>
                <script dangerouslySetInnerHTML={{ __html: themeScript }} />
            </head>
            <body className="font-sans antialiased">
                {children}
                <Toaster />
            </body>
        </html>
    );
}
