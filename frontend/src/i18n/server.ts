import 'server-only';

import { cache } from 'react';
import { cookies, headers } from 'next/headers';
import { getAppearance } from '@/lib/server/appearance';
import { tryMe } from '@/lib/server/auth';
import { isLocale, type Locale } from './config';
import { dictionaries } from './messages';
import { createTranslator, type Translate } from './translate';

/** Language picked on the sign-in screens, before anyone is signed in. */
export const LOCALE_COOKIE = 'messora_locale';

/**
 * The language for this request: the signed-in user's choice, then the
 * language picked on the sign-in screens, then the workspace default, then
 * the browser's preference.
 */
export const getLocale = cache(async (): Promise<Locale> => {
    const r = await tryMe();
    if (r.me && isLocale(r.me.user.locale)) return r.me.user.locale;
    const picked = (await cookies()).get(LOCALE_COOKIE)?.value;
    if (isLocale(picked)) return picked;
    const appearance = await getAppearance();
    if (r.me || appearance.default_locale !== 'en') {
        return appearance.default_locale;
    }
    const accept = (await headers()).get('accept-language') ?? '';
    return /^\s*tr\b/i.test(accept) ? 'tr' : appearance.default_locale;
});

/** t() for server components. */
export const getT = cache(async (): Promise<Translate> => {
    const locale = await getLocale();
    return createTranslator(dictionaries[locale], locale);
});
