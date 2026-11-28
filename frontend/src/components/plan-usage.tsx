'use client';

import { AlertTriangle, Check, Minus, Sparkles } from 'lucide-react';
import { useFormatDate } from '@/components/display-preferences';
import { Badge } from '@/components/ui/badge';
import { useLocale, useT } from '@/i18n/client';
import type { MessageKey } from '@/i18n/translate';
import {
    formatBytes,
    formatCredits,
    PLAN_FEATURES,
    usagePercent,
    usageTone,
} from '@/lib/plan';
import type { Schemas } from '@/lib/types';
import { cn } from '@/lib/utils';

type Overview = Schemas['PlanOverview'];

/** The plan's name in the panel's language. */
export function usePlanName() {
    const locale = useLocale();
    const t = useT();
    return (p: { key: string; name_tr: string; name_en: string }) =>
        (locale === 'tr' ? p.name_tr : p.name_en) ||
        (p.key === 'default' ? t('plans.defaultPlan') : p.key);
}

/** A labelled bar of used against limit (0 = no limit). */
export function UsageBar({
    label,
    used,
    limit,
    format = (n) => String(n),
    unlimitedLabel,
    testId,
}: {
    label: string;
    used: number;
    limit: number;
    format?: (n: number) => string;
    unlimitedLabel?: string;
    testId?: string;
}) {
    const t = useT();
    const tone = usageTone(used, limit);
    const pct = tone === 'unlimited' ? 0 : usagePercent(used, limit);
    return (
        <div className="grid gap-1.5" data-testid={testId} data-tone={tone}>
            <div className="flex items-baseline justify-between gap-3 text-sm">
                <span className="font-medium">{label}</span>
                <span
                    className={cn(
                        'text-muted-foreground tabular-nums',
                        tone === 'over' && 'font-medium text-warning',
                    )}
                >
                    {tone === 'unlimited'
                        ? `${format(used)} · ${unlimitedLabel ?? t('plans.unlimited')}`
                        : t('plans.of', {
                              used: format(used),
                              limit: format(limit),
                          })}
                </span>
            </div>
            <div className="h-2 overflow-hidden rounded-full bg-muted">
                <div
                    className={cn(
                        'h-full rounded-full transition-all',
                        tone === 'over' && 'bg-warning',
                        tone === 'high' && 'bg-amber-500',
                        tone === 'ok' && 'bg-primary',
                        tone === 'unlimited' && 'bg-primary/30',
                    )}
                    style={{
                        width: tone === 'unlimited' ? '100%' : `${pct}%`,
                    }}
                />
            </div>
            {tone === 'over' && (
                <p className="flex items-center gap-1.5 text-xs text-warning">
                    <AlertTriangle className="size-3.5" aria-hidden />
                    {t('plans.overLimit')}
                </p>
            )}
        </div>
    );
}

/** Every limit of the plan next to its usage, with API calls and credits. */
export function PlanUsage({ overview: o }: { overview: Overview }) {
    const t = useT();
    const locale = useLocale();
    const fmtDate = useFormatDate();
    const num = (n: number) => new Intl.NumberFormat(locale).format(n);
    const c = o.ai_credits;
    return (
        <div className="grid gap-6">
            <div className="grid gap-5 md:grid-cols-2" data-testid="plan-usage">
                {o.usage.map((u) => (
                    <UsageBar
                        key={u.resource}
                        testId={`usage-${u.resource}`}
                        label={t(`plans.resources.${u.resource}` as MessageKey)}
                        used={u.used}
                        limit={u.unlimited ? 0 : u.limit}
                        format={
                            u.resource === 'storage_bytes'
                                ? (n) => formatBytes(n, locale)
                                : num
                        }
                        unlimitedLabel={
                            u.resource === 'contacts'
                                ? t('plans.fairUse')
                                : undefined
                        }
                    />
                ))}
                {o.features.public_api ? (
                    <UsageBar
                        testId="usage-api_calls"
                        label={t('plans.apiCalls')}
                        used={o.api_calls.used}
                        limit={o.api_calls.unlimited ? 0 : o.api_calls.limit}
                        format={num}
                    />
                ) : (
                    <div className="grid gap-1.5 text-sm">
                        <span className="font-medium">
                            {t('plans.apiCalls')}
                        </span>
                        <span className="text-muted-foreground">
                            {t('plans.apiCallsNone')}
                        </span>
                    </div>
                )}
            </div>
            <div
                className="grid gap-3 rounded-xl border bg-muted/20 p-4"
                data-testid="plan-credits"
            >
                <div className="flex flex-wrap items-center justify-between gap-2">
                    <span className="flex items-center gap-2 font-medium">
                        <Sparkles className="size-4 text-primary" aria-hidden />
                        {t('plans.credits.title')}
                    </span>
                    <span className="text-xs text-muted-foreground">
                        {t('plans.credits.resets', {
                            date: fmtDate(c.resets_at),
                        })}
                    </span>
                </div>
                {c.unlimited ? (
                    <p className="text-sm text-muted-foreground">
                        {t('plans.credits.unlimited')}
                    </p>
                ) : (
                    <UsageBar
                        label={t('plans.credits.remaining', {
                            remaining: formatCredits(c.remaining, locale),
                            granted: formatCredits(c.granted, locale),
                        })}
                        used={c.used}
                        limit={c.granted}
                        format={(n) => formatCredits(n, locale)}
                    />
                )}
                <p className="text-xs text-muted-foreground">
                    {t('plans.credits.usedThisMonth', {
                        used: formatCredits(c.used_this_month, locale),
                    })}{' '}
                    · {t('plans.credits.hint')}
                </p>
                {c.exhausted && (
                    <p
                        className="flex items-center gap-1.5 text-sm font-medium text-warning"
                        data-testid="credits-exhausted"
                    >
                        <AlertTriangle className="size-4" aria-hidden />
                        {t('plans.credits.exhausted', {
                            date: fmtDate(c.resets_at),
                        })}
                    </p>
                )}
            </div>
        </div>
    );
}

/** Included / not included for each feature. */
export function PlanFeatureList({
    features,
}: {
    features: Schemas['PlanFeatures'];
}) {
    const t = useT();
    return (
        <ul className="grid gap-3 sm:grid-cols-2" data-testid="plan-features">
            {PLAN_FEATURES.map((f) => {
                const on = features[f];
                return (
                    <li
                        key={f}
                        className="flex items-start gap-3"
                        data-feature={f}
                        data-included={on}
                    >
                        <span
                            className={cn(
                                'mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full',
                                on
                                    ? 'bg-success/15 text-success'
                                    : 'bg-muted text-muted-foreground',
                            )}
                        >
                            {on ? (
                                <Check className="size-3.5" />
                            ) : (
                                <Minus className="size-3.5" />
                            )}
                        </span>
                        <span className="grid gap-0.5">
                            <span className="flex items-center gap-2 text-sm font-medium">
                                {t(`plans.features.${f}.name`)}
                                <Badge
                                    variant="outline"
                                    className="text-[10px]"
                                >
                                    {on
                                        ? t('plans.included')
                                        : t('plans.notIncluded')}
                                </Badge>
                            </span>
                            <span className="text-xs text-muted-foreground">
                                {t(`plans.features.${f}.hint`)}
                            </span>
                        </span>
                    </li>
                );
            })}
        </ul>
    );
}
