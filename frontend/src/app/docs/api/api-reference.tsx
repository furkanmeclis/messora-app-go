'use client';

import dynamic from 'next/dynamic';
import { useMemo } from 'react';
import { useTheme } from 'next-themes';
import type { AnyApiReferenceConfiguration } from '@scalar/api-reference-react';
import '@scalar/api-reference-react/style.css';
import type { Locale } from '@/i18n/config';
import { scalarTr } from './scalar-tr';

// Scalar mounts a Vue app into the page; it only runs in the browser.
const ApiReferenceReact = dynamic(
    () =>
        import('@scalar/api-reference-react').then((m) => m.ApiReferenceReact),
    { ssr: false },
);

/** The public document, served anonymously by the Go API. */
export const SPEC_URL = '/api/v1/openapi.json';

/**
 * Scalar's colours follow the workspace theme: its variables point at the
 * panel's tokens, which the root layout already sets for light and dark.
 */
const THEME_CSS = `
.light-mode, .dark-mode {
  --scalar-font: var(--app-font);
  --scalar-font-code: ui-monospace, SFMono-Regular, Menlo, Consolas, 'Liberation Mono', monospace;
  --scalar-radius: calc(var(--radius) * 0.6);
  --scalar-radius-lg: var(--radius);
  --scalar-radius-xl: calc(var(--radius) * 1.4);
  --scalar-background-1: var(--background);
  --scalar-background-2: var(--muted);
  --scalar-background-3: color-mix(in oklab, var(--muted), var(--foreground) 8%);
  --scalar-background-accent: color-mix(in oklab, var(--primary) 12%, transparent);
  --scalar-color-1: var(--foreground);
  --scalar-color-2: var(--muted-foreground);
  --scalar-color-3: color-mix(in oklab, var(--muted-foreground) 80%, transparent);
  --scalar-color-accent: var(--primary);
  --scalar-border-color: var(--border);
  --scalar-sidebar-background-1: var(--sidebar);
  --scalar-sidebar-color-1: var(--sidebar-foreground);
  --scalar-sidebar-color-2: color-mix(in oklab, var(--sidebar-foreground) 70%, transparent);
  --scalar-sidebar-border-color: var(--sidebar-border);
  --scalar-sidebar-item-hover-background: var(--sidebar-accent);
  --scalar-sidebar-item-active-background: var(--sidebar-accent);
  --scalar-sidebar-color-active: var(--sidebar-accent-foreground);
  --scalar-sidebar-search-background: var(--background);
  --scalar-sidebar-search-border-color: var(--sidebar-border);
}
`;

export function ApiReference({ locale }: { locale: Locale }) {
    const { resolvedTheme } = useTheme();
    const dark = resolvedTheme === 'dark';
    const configuration = useMemo<AnyApiReferenceConfiguration>(
        () => ({
            url: SPEC_URL,
            metaData: { title: 'Messora API' },
            theme: 'none',
            customCss: THEME_CSS,
            // Everything is served from this origin: no remote fonts, no
            // telemetry, no Scalar cloud features or request proxy.
            withDefaultFonts: false,
            telemetry: false,
            showDeveloperTools: 'never',
            showToolbar: 'never',
            agent: { disabled: true },
            mcp: { disabled: true },
            hideClientButton: false,
            persistAuth: false,
            documentDownloadType: 'json',
            forceDarkModeState: dark ? 'dark' : 'light',
            darkMode: dark,
            hideDarkModeToggle: true,
            showExtensions: ['x-required-scopes'],
            defaultHttpClient: { targetKey: 'shell', clientKey: 'curl' },
            featuredClients: [
                'shell/curl',
                'js/fetch',
                'python/requests',
                'go/native',
                'php/curl',
            ],
            authentication: {
                preferredSecurityScheme: 'apiKey',
                securitySchemes: { apiKey: { token: 'msk_live_YOUR_KEY' } },
            },
            locale,
            ...(locale === 'tr' ? { translations: scalarTr } : {}),
        }),
        [dark, locale],
    );
    return (
        <div data-testid="api-reference">
            <ApiReferenceReact configuration={configuration} />
        </div>
    );
}
