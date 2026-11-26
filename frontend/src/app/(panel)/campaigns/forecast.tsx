'use client';

import { useEffect, useState } from 'react';
import { CalendarClock, Gauge, Receipt, TrendingUp } from 'lucide-react';
import { useFormatDateTime } from '@/components/display-preferences';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Spinner } from '@/components/ui/spinner';
import { useT } from '@/i18n/client';
import { NUMBER_LOCALE } from '@/lib/format';
import type { ActionResult, Schemas, Template } from '@/lib/types';
import { forecastAction, type CampaignForecast } from './analytics-actions';
import { SourceBadge } from './[id]/analysis';

const n = (v: number) => v.toLocaleString(NUMBER_LOCALE);
const pct = (r: number | null | undefined) =>
    r == null
        ? '—'
        : `%${(r * 100).toLocaleString(NUMBER_LOCALE, { maximumFractionDigits: 1 })}`;

/** The quick reply button texts of a template (keys of a reaction map). */
export function quickRepliesOf(t: Template): string[] {
    const out: string[] = [];
    for (const c of t.components) {
        const buttons = (c as { buttons?: { type?: string; text?: string }[] })
            .buttons;
        for (const b of buttons ?? []) {
            if (b.type?.toUpperCase() === 'QUICK_REPLY' && b.text)
                out.push(b.text);
        }
    }
    return out;
}

function duration(sec: number): string {
    const d = Math.floor(sec / 86400);
    const h = Math.floor((sec % 86400) / 3600);
    const m = Math.ceil((sec % 3600) / 60);
    if (d > 0) return `${d} g ${h} sa`;
    if (h > 0) return `${h} sa ${m} dk`;
    return sec < 60 ? `${Math.max(1, Math.round(sec))} sn` : `${m} dk`;
}

/**
 * The review step's pre-launch analysis: eligible audience, duration and
 * finish with the number's real capacity and the messaging limit, the Meta
 * cost from the rate card and, only with enough history, expected rates.
 */
export function ForecastPanel({
    input,
}: {
    input: Schemas['CampaignForecastRequest'];
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const key = JSON.stringify(input);
    const [result, setResult] = useState<{
        key: string;
        r: ActionResult<CampaignForecast>;
    } | null>(null);
    useEffect(() => {
        let cancelled = false;
        const id = setTimeout(() => {
            forecastAction(
                JSON.parse(key) as Schemas['CampaignForecastRequest'],
            ).then((r) => {
                if (!cancelled) setResult({ key, r });
            });
        }, 300);
        return () => {
            cancelled = true;
            clearTimeout(id);
        };
    }, [key]);
    const r = result?.key === key ? result.r : undefined;
    if (!r) {
        return (
            <p
                className="flex items-center gap-2 text-sm text-muted-foreground"
                role="status"
                data-testid="campaign-forecast"
                data-state="loading"
            >
                <Spinner /> {t('analytics.forecast.loading')}
            </p>
        );
    }
    if (!r.ok) {
        return (
            <Alert
                variant="destructive"
                data-testid="campaign-forecast"
                data-state="error"
            >
                <AlertDescription>
                    {t('analytics.forecast.failed')} {r.message}
                </AlertDescription>
            </Alert>
        );
    }
    const f = r.data;
    const rates = f.rates;
    return (
        <section
            className="space-y-4 rounded-lg border p-4"
            data-testid="campaign-forecast"
            data-state="ready"
        >
            <div className="flex items-center justify-between gap-2">
                <h3 className="font-medium">{t('analytics.forecast.title')}</h3>
                <SourceBadge source="system" />
            </div>
            <dl className="grid gap-4 text-sm sm:grid-cols-2">
                <div className="space-y-1">
                    <dt className="flex items-center gap-2 text-muted-foreground">
                        <Gauge className="size-4" />{' '}
                        {t('analytics.forecast.eligible')}
                    </dt>
                    <dd className="text-lg font-semibold tabular-nums">
                        {n(f.eligible)}
                        <span className="ml-2 text-sm font-normal text-muted-foreground">
                            {t('analytics.forecast.blocked')}: {n(f.blocked)}
                        </span>
                    </dd>
                </div>
                <div className="space-y-1">
                    <dt className="flex items-center gap-2 text-muted-foreground">
                        <CalendarClock className="size-4" />{' '}
                        {t('analytics.forecast.duration')}
                    </dt>
                    <dd data-testid="forecast-duration">
                        {f.duration_seconds == null ? (
                            t('analytics.forecast.noCapacity')
                        ) : (
                            <>
                                <span className="text-lg font-semibold">
                                    {duration(f.duration_seconds)}
                                </span>
                                {f.finish_at && (
                                    <span className="ml-2 text-muted-foreground">
                                        {t('analytics.forecast.finish')}:{' '}
                                        {formatDateTime(f.finish_at)}
                                    </span>
                                )}
                            </>
                        )}
                        <p className="text-xs text-muted-foreground">
                            {t('analytics.forecast.sharePerSecond', {
                                rate: f.capacity.share_mps.toLocaleString(
                                    NUMBER_LOCALE,
                                    { maximumFractionDigits: 1 },
                                ),
                                capacity:
                                    f.capacity.campaign_capacity_mps.toLocaleString(
                                        NUMBER_LOCALE,
                                        { maximumFractionDigits: 1 },
                                    ),
                            })}
                            {f.capacity.others.length > 0 &&
                                ` ${t('analytics.forecast.others', { count: f.capacity.others.length })}`}
                        </p>
                        <p
                            className="text-xs text-muted-foreground"
                            data-testid="forecast-limit"
                        >
                            {f.limit.limit === 0
                                ? t('analytics.forecast.noLimit')
                                : f.limit.windows > 1
                                  ? t('analytics.forecast.windows', {
                                        remaining: n(f.limit.remaining),
                                        windows: f.limit.windows,
                                    })
                                  : t('analytics.forecast.limitFits', {
                                        remaining: n(f.limit.remaining),
                                    })}
                        </p>
                    </dd>
                </div>
                <div className="space-y-1">
                    <dt className="flex items-center gap-2 text-muted-foreground">
                        <Receipt className="size-4" />{' '}
                        {t('analytics.forecast.cost')}
                    </dt>
                    <dd data-testid="forecast-cost">
                        <span className="text-lg font-semibold tabular-nums">
                            {Number(f.cost.amount).toLocaleString(
                                NUMBER_LOCALE,
                                { maximumFractionDigits: 4 },
                            )}{' '}
                            {f.cost.currency}
                        </span>
                        {f.cost.not_in_rate_card > 0 && (
                            <p className="text-xs text-amber-600 dark:text-amber-400">
                                {t('analytics.forecast.notInRateCard', {
                                    count: n(f.cost.not_in_rate_card),
                                })}
                            </p>
                        )}
                        {f.cost.free > 0 && (
                            <p className="text-xs text-muted-foreground">
                                {t('analytics.forecast.free', {
                                    count: n(f.cost.free),
                                })}
                            </p>
                        )}
                        <p className="text-xs text-muted-foreground">
                            {t('analytics.forecast.costHint')}
                        </p>
                    </dd>
                </div>
                <div className="space-y-1">
                    <dt className="flex items-center gap-2 text-muted-foreground">
                        <TrendingUp className="size-4" />{' '}
                        {t('analytics.forecast.rates')}
                    </dt>
                    <dd data-testid="forecast-rates" data-status={rates.status}>
                        {rates.status !== 'ok' ? (
                            <p className="text-muted-foreground">
                                {t('analytics.forecast.insufficient', {
                                    campaigns: rates.campaigns,
                                    campaignsNeeded: rates.campaigns_needed,
                                    recipients: n(rates.recipients),
                                    recipientsNeeded: n(
                                        rates.recipients_needed,
                                    ),
                                })}
                            </p>
                        ) : (
                            <>
                                <ul className="space-y-0.5">
                                    {(
                                        [
                                            [
                                                'delivery',
                                                rates.delivery_rate,
                                                f.expected.delivered,
                                            ],
                                            [
                                                'read',
                                                rates.read_rate,
                                                f.expected.read,
                                            ],
                                            [
                                                'reply',
                                                rates.reply_rate,
                                                f.expected.replied,
                                            ],
                                            [
                                                'positive',
                                                rates.positive_rate,
                                                f.expected.positive,
                                            ],
                                            [
                                                'opt_out',
                                                rates.opt_out_rate,
                                                f.expected.opted_out,
                                            ],
                                        ] as const
                                    ).map(([k, rate, count]) => (
                                        <li
                                            key={k}
                                            className="flex justify-between gap-2"
                                        >
                                            <span className="text-muted-foreground">
                                                {t(`analytics.rates.${k}`)}
                                            </span>
                                            <span className="tabular-nums">
                                                {pct(rate)}
                                                {count != null &&
                                                    ` · ${t('analytics.forecast.expected')} ${n(count)}`}
                                            </span>
                                        </li>
                                    ))}
                                </ul>
                                <p className="text-xs text-muted-foreground">
                                    {t('analytics.forecast.basedOn', {
                                        count: rates.campaigns,
                                        recipients: n(rates.recipients),
                                    })}
                                </p>
                            </>
                        )}
                    </dd>
                </div>
            </dl>
            <p className="text-xs text-muted-foreground">
                {t('analytics.forecast.snapshotNote')}
            </p>
        </section>
    );
}
