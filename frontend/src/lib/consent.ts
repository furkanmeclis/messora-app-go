import type { Schemas } from './types';

/**
 * Consent vocabulary shared by contacts, campaigns, inbox and settings.
 * Mirrors internal/modules/consent (docs/CONSENT.md); pure, so it can be
 * unit tested.
 */

export type ConsentStatus = Schemas['ConsentStatus'];
export type ConsentCategory = 'marketing' | 'service';
export type PreflightReason = Schemas['PreflightReason']['reason'];

export const CONSENT_STATUSES: ConsentStatus[] = [
    'opted_in',
    'unknown',
    'opt_out_requested',
    'opted_out',
    'resubscribed',
];

export const CONSENT_CATEGORIES: ConsentCategory[] = ['marketing', 'service'];

/** Why consent keeps a message from a contact (campaign skip reasons too). */
export const CONSENT_REASONS: PreflightReason[] = [
    'opted_out',
    'meta_marketing_stopped',
    'undeliverable',
    'opt_out_requested',
    'consent_unknown',
    'replied_after_opt_out',
];

export function isConsentStatus(v: unknown): v is ConsentStatus {
    return CONSENT_STATUSES.includes(v as ConsentStatus);
}

export function isConsentReason(v: unknown): v is PreflightReason {
    return CONSENT_REASONS.includes(v as PreflightReason);
}

/** The badge tone of a status: may receive, unclear, or blocked. */
export function statusTone(
    s: ConsentStatus,
): 'ok' | 'neutral' | 'warn' | 'bad' {
    switch (s) {
        case 'opted_in':
        case 'resubscribed':
            return 'ok';
        case 'opt_out_requested':
            return 'warn';
        case 'opted_out':
            return 'bad';
        default:
            return 'neutral';
    }
}

/**
 * Who said "no" (opted out, stopped marketing in WhatsApp or asked to stop),
 * as opposed to contacts blocked for other reasons (undeliverable number,
 * unknown consent under a blocking policy).
 */
export function refusedCount(reasons: { reason: string; count: number }[]) {
    return reasons
        .filter(
            (r) =>
                r.reason === 'opted_out' ||
                r.reason === 'meta_marketing_stopped' ||
                r.reason === 'opt_out_requested',
        )
        .reduce((sum, r) => sum + r.count, 0);
}

/** Puts hard blocks first, then the larger groups. */
export function sortReasons<T extends { count: number; hard: boolean }>(
    list: T[],
): T[] {
    return [...list].sort(
        (a, b) => Number(b.hard) - Number(a.hard) || b.count - a.count,
    );
}

/** Cleans a keyword list: trimmed, without blanks or repeats (any case). */
export function cleanKeywords(list: string[]): string[] {
    const out: string[] = [];
    const seen = new Set<string>();
    for (const raw of list) {
        const w = raw.trim();
        const key = w.toLocaleLowerCase();
        if (!w || seen.has(key)) continue;
        seen.add(key);
        out.push(w);
    }
    return out;
}
