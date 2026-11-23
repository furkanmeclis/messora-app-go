export const locales = ['en', 'tr'] as const;
export type Locale = (typeof locales)[number];
export const defaultLocale: Locale = 'en';

export function isLocale(v: unknown): v is Locale {
    return typeof v === 'string' && (locales as readonly string[]).includes(v);
}

/** BCP 47 tag for Intl formatting. */
export function intlLocale(l: Locale): string {
    return l === 'tr' ? 'tr-TR' : 'en-GB';
}

/** Names shown in language pickers, each in its own language. */
export const localeNames: Record<Locale, string> = {
    en: 'English',
    tr: 'Türkçe',
};
