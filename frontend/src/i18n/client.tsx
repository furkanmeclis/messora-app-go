'use client';

import { createContext, useContext, useMemo } from 'react';
import type { Locale } from './config';
import { dictionaries } from './messages';
import { createTranslator, type Translate } from './translate';

const LocaleContext = createContext<Locale>('en');

/** Provides the request's language to client components. */
export function I18nProvider({
    locale,
    children,
}: {
    locale: Locale;
    children: React.ReactNode;
}) {
    return (
        <LocaleContext.Provider value={locale}>
            {children}
        </LocaleContext.Provider>
    );
}

export function useLocale(): Locale {
    return useContext(LocaleContext);
}

/** t() for client components. */
export function useT(): Translate {
    const locale = useLocale();
    return useMemo(
        () => createTranslator(dictionaries[locale], locale),
        [locale],
    );
}
