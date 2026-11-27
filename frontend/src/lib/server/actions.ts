import 'server-only';

import { redirect } from 'next/navigation';
import { ApiError } from './api';
import { clearSession } from './session';
import type { ActionResult } from '@/lib/types';
import { getLocale } from '@/i18n/server';
import { dictionaries } from '@/i18n/messages';

/**
 * The message for an API error in the user's language. English keeps the
 * API's own (more specific) wording; other languages translate by code and
 * keep validation details.
 */
export async function errorMessage(err: ApiError): Promise<string> {
    const locale = await getLocale();
    if (locale === 'en') return err.message;
    const errors = dictionaries[locale].errors as Record<string, string>;
    const msg = errors[err.code];
    if (!msg) return err.message;
    return msg.replace('{detail}', err.message);
}

/**
 * Runs fn and converts API errors into an ActionResult with per-field
 * messages. An ended session redirects to the sign-in page.
 */
export async function runAction<T>(
    fn: () => Promise<T>,
    message?: string,
): Promise<ActionResult<T>> {
    try {
        return { ok: true, data: await fn(), message };
    } catch (err) {
        if (err instanceof ApiError) {
            if (
                err.code === 'SESSION_INVALID' ||
                (err.status === 401 && err.code === 'UNAUTHORIZED')
            ) {
                await clearSession();
                redirect('/login');
            }
            const fieldErrors: Record<string, string> = {};
            for (const d of err.details) {
                if (d.field && !fieldErrors[d.field]) {
                    fieldErrors[d.field] = d.message ?? 'Invalid value';
                }
            }
            let msg = await errorMessage(err);
            if (err.retryAfter && err.status === 429) {
                msg += ` (retry in ${Math.ceil(err.retryAfter / 60)} min)`;
            }
            return { ok: false, code: err.code, message: msg, fieldErrors };
        }
        throw err;
    }
}

/** Reads a trimmed string field from FormData. */
export function field(form: FormData, name: string): string {
    const v = form.get(name);
    return typeof v === 'string' ? v.trim() : '';
}

/** Same-origin relative path for post-login redirects (no open redirects). */
export function safeNext(raw: string | null | undefined): string {
    if (!raw || !raw.startsWith('/') || raw.startsWith('//')) return '/';
    return raw;
}
