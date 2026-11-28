/**
 * Plan helpers shared by the workspace panel and the platform admin panel
 * (Phase 17 WP8, docs/ENTITLEMENTS.md). Pure functions: unit-tested.
 */

export const PLAN_FEATURES = [
    'custom_smtp',
    'byo_ai_keys',
    'custom_domain',
    'public_api',
    'webhooks',
    'ai_agents',
] as const;
export type PlanFeature = (typeof PLAN_FEATURES)[number];

export const PLAN_LIMIT_KEYS = [
    'users',
    'phone_numbers',
    'ai_agents',
    'automation_flows',
    'ai_credits_per_month',
    'api_calls_per_month',
    'storage_bytes',
    'webhook_endpoints',
    'webhook_deliveries_per_minute',
    'campaign_messages_per_second',
    'contacts',
    'session_requests_per_minute',
    'api_key_requests_per_minute',
] as const;
export type PlanLimitKey = (typeof PLAN_LIMIT_KEYS)[number];

type MeLike = {
    tenant?: { plan?: { features: readonly string[] } | null } | null;
};

/**
 * Whether the workspace's plan includes feature. Without plan information
 * (older API, platform host) everything counts as included: the API checks
 * the plan anyway, the panel only hides what cannot be used.
 */
export function hasFeature(me: MeLike, feature: PlanFeature): boolean {
    const plan = me.tenant?.plan;
    if (!plan) return true;
    return plan.features.includes(feature);
}

/** Share of a limit in use, 0–100 (0 for unlimited limits). */
export function usagePercent(used: number, limit: number): number {
    if (!limit || limit <= 0) return 0;
    return Math.min(100, Math.max(0, Math.round((used / limit) * 100)));
}

/** How a usage bar is drawn: over the limit, close to it, or fine. */
export function usageTone(
    used: number,
    limit: number,
): 'over' | 'high' | 'ok' | 'unlimited' {
    if (!limit || limit <= 0) return 'unlimited';
    if (used > limit) return 'over';
    if (used >= limit * 0.8) return 'high';
    return 'ok';
}

/** Bytes for people (1024-based, one decimal from KB on). */
export function formatBytes(n: number, locale = 'en'): string {
    const units = ['B', 'KB', 'MB', 'GB', 'TB'];
    let v = Math.max(0, n);
    let i = 0;
    while (v >= 1024 && i < units.length - 1) {
        v /= 1024;
        i++;
    }
    const digits = i === 0 || v >= 100 ? 0 : 1;
    const num = new Intl.NumberFormat(locale, {
        maximumFractionDigits: digits,
        minimumFractionDigits: 0,
    }).format(v);
    return `${num} ${units[i]}`;
}

/** Credits with at most three decimals. */
export function formatCredits(n: number, locale = 'en'): string {
    return new Intl.NumberFormat(locale, { maximumFractionDigits: 3 }).format(
        n,
    );
}

/** A decimal price string in its currency (whole units when exact). */
export function formatPrice(
    price: string | null | undefined,
    currency: string,
    locale = 'tr',
): string | null {
    if (price === null || price === undefined || price === '') return null;
    const v = Number(price);
    if (!Number.isFinite(v)) return null;
    return new Intl.NumberFormat(locale, {
        style: 'currency',
        currency,
        maximumFractionDigits: Number.isInteger(v) ? 0 : 2,
    }).format(v);
}

type Detail = {
    field?: string | null;
    code?: string | null;
    message?: string | null;
};

export type PlanError =
    | {
          kind: 'limit';
          resource: string;
          limit: number | null;
          current: number | null;
      }
    | { kind: 'feature'; feature: string };

/**
 * Reads the plan errors of the API: 409 PLAN_LIMIT_REACHED (details
 * field "limit"/"current", code = resource) and 403 FEATURE_NOT_IN_PLAN
 * (details field "feature", code = feature). Anything else: null.
 */
export function planError(
    code: string | undefined,
    details: readonly Detail[] | undefined | null,
): PlanError | null {
    const ds = details ?? [];
    if (code === 'PLAN_LIMIT_REACHED') {
        const lim = ds.find((d) => d.field === 'limit');
        const cur = ds.find((d) => d.field === 'current');
        const num = (s: string | null | undefined) => {
            const v = Number(s);
            return s !== undefined &&
                s !== null &&
                s !== '' &&
                Number.isFinite(v)
                ? v
                : null;
        };
        return {
            kind: 'limit',
            resource: lim?.code ?? cur?.code ?? '',
            limit: num(lim?.message),
            current: num(cur?.message),
        };
    }
    if (code === 'FEATURE_NOT_IN_PLAN') {
        const f = ds.find((d) => d.field === 'feature');
        return { kind: 'feature', feature: f?.code ?? '' };
    }
    return null;
}

/** Where the panel explains plans and limits. */
export const PLAN_PAGE = '/settings/plan';

/**
 * Parses the overrides textarea of the admin panel: a JSON object whose
 * keys are limits or "features" (an object of booleans).
 */
export function parseOverrides(
    raw: string,
): { ok: true; value: Record<string, unknown> } | { ok: false; error: string } {
    const s = raw.trim();
    if (s === '') return { ok: true, value: {} };
    let v: unknown;
    try {
        v = JSON.parse(s);
    } catch {
        return { ok: false, error: 'json' };
    }
    if (v === null || typeof v !== 'object' || Array.isArray(v)) {
        return { ok: false, error: 'object' };
    }
    for (const [k, val] of Object.entries(v)) {
        if (k === 'features') {
            if (val === null || typeof val !== 'object' || Array.isArray(val))
                return { ok: false, error: 'features' };
            for (const [f, b] of Object.entries(val)) {
                if (!(PLAN_FEATURES as readonly string[]).includes(f))
                    return { ok: false, error: `unknown:${f}` };
                if (typeof b !== 'boolean')
                    return { ok: false, error: `features.${f}` };
            }
            continue;
        }
        if (!(PLAN_LIMIT_KEYS as readonly string[]).includes(k))
            return { ok: false, error: `unknown:${k}` };
        if (typeof val !== 'number' || !Number.isInteger(val) || val < 0)
            return { ok: false, error: k };
    }
    return { ok: true, value: v as Record<string, unknown> };
}
