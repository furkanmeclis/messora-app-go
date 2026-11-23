'use client';

import { ThemeProvider as NextThemesProvider } from 'next-themes';

/** Light / dark / system theme, persisted in localStorage by next-themes. */
export function ThemeProvider({
    children,
    nonce,
    defaultTheme = 'system',
}: {
    children: React.ReactNode;
    nonce?: string;
    /** The workspace's default scheme; each person may still choose. */
    defaultTheme?: 'light' | 'dark' | 'system';
}) {
    return (
        <NextThemesProvider
            nonce={nonce}
            attribute="class"
            defaultTheme={defaultTheme}
            enableSystem
            disableTransitionOnChange
        >
            {children}
        </NextThemesProvider>
    );
}
