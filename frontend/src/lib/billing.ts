/**
 * Billing helpers shared by the workspace panel and the platform admin
 * panel (Phase 17 WP9, docs/BILLING.md). Pure functions: unit-tested.
 *
 * Amounts travel as TL decimal strings with two decimals ("3588.00");
 * every computation here works in integer kuruş and rounds half up, the
 * same rule as the API.
 */

/** Kuruş of a TL decimal string ("3588.00", "2990", "1.234,56" is not). */
export function toKurus(amount: string | null | undefined): number | null {
    if (amount === null || amount === undefined) return null;
    const s = amount.trim().replace(',', '.');
    if (!/^-?\d+(\.\d{1,2})?$/.test(s)) return null;
    const neg = s.startsWith('-');
    const [whole, frac = ''] = s.replace('-', '').split('.');
    const k = Number(whole) * 100 + Number((frac + '00').slice(0, 2));
    return neg ? -k : k;
}

/** A TL decimal string with two decimals from kuruş ("3588.00"). */
export function fromKurus(k: number): string {
    const neg = k < 0;
    const a = Math.abs(Math.round(k));
    const s = `${Math.floor(a / 100)}.${String(a % 100).padStart(2, '0')}`;
    return neg ? `-${s}` : s;
}

/** The VAT of net kuruş at a percentage, rounded half up to a kuruş. */
export function vatKurus(net: number, ratePercent: string | number): number {
    const bp = Math.round(Number(ratePercent) * 100);
    if (!Number.isFinite(bp)) return 0;
    const num = net * bp;
    // Half up for positive amounts, away from zero for negative ones.
    const q = Math.floor((2 * Math.abs(num) + 10_000) / 20_000);
    return num < 0 ? -q : q;
}

/** Net, VAT and total of a net TL amount. */
export function withVAT(
    net: string,
    ratePercent: string | number,
): { net: number; vat: number; total: number } | null {
    const n = toKurus(net);
    if (n === null) return null;
    const v = vatKurus(n, ratePercent);
    return { net: n, vat: v, total: n + v };
}

/** Kuruş (or a TL decimal string) as Turkish money: "3.588,00 TL". */
export function formatTL(
    value: number | string | null | undefined,
    locale = 'tr',
): string {
    const k = typeof value === 'number' ? value : toKurus(value ?? null);
    if (k === null) return '—';
    const num = new Intl.NumberFormat(locale === 'tr' ? 'tr-TR' : 'en-GB', {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
    }).format(k / 100);
    return locale === 'tr' ? `${num} TL` : `TRY ${num}`;
}

/** An IBAN in groups of four: "TR33 0006 1005 …". */
export function formatIBAN(iban: string): string {
    const s = iban.replace(/\s+/g, '').toUpperCase();
    return s.replace(/(.{4})(?=.)/g, '$1 ');
}

/** The billing page of a workspace. */
export const BILLING_PAGE = '/settings/billing';

/** Billing errors that point the user to the billing page. */
export const BILLING_ERRORS = [
    'SUBSCRIPTION_READ_ONLY',
    'MESSAGE_BALANCE_EXHAUSTED',
    'BILLING_PROFILE_REQUIRED',
] as const;

export function isBillingError(code: string | null | undefined): boolean {
    return !!code && (BILLING_ERRORS as readonly string[]).includes(code);
}

/** Whole days from now until iso (negative once passed). */
export function daysUntil(iso: string | null | undefined, now = Date.now()) {
    if (!iso) return null;
    const t = Date.parse(iso);
    if (Number.isNaN(t)) return null;
    return Math.ceil((t - now) / 86_400_000);
}

type BillingMe = {
    subscription_status?: string;
    read_only?: boolean;
    trial_ends_at?: string | null;
    grace_ends_at?: string | null;
    message_balance_exhausted?: boolean;
};

/** Which billing banner the panel shows (most urgent first), or null. */
export function billingBanner(
    plan: BillingMe | null | undefined,
    now = Date.now(),
):
    | { kind: 'read_only' }
    | { kind: 'past_due'; until: string | null }
    | { kind: 'trial'; until: string; days: number }
    | { kind: 'balance' }
    | null {
    if (!plan) return null;
    if (plan.read_only) return { kind: 'read_only' };
    if (plan.subscription_status === 'past_due') {
        return { kind: 'past_due', until: plan.grace_ends_at ?? null };
    }
    if (plan.subscription_status === 'trial' && plan.trial_ends_at) {
        const d = daysUntil(plan.trial_ends_at, now);
        if (d !== null && d >= 0 && d <= 3) {
            return { kind: 'trial', until: plan.trial_ends_at, days: d };
        }
    }
    if (plan.message_balance_exhausted) return { kind: 'balance' };
    return null;
}
