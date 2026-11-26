'use client';

import { useRouter } from 'next/navigation';
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from 'recharts';
import {
    Activity,
    AlertTriangle,
    Coins,
    MessageSquareText,
    Repeat,
    Timer,
    Wallet,
    XCircle,
    type LucideIcon,
} from 'lucide-react';
import {
    ClientDataTable,
    type ClientColumn,
} from '@/components/client-data-table';
import { useFormatDate } from '@/components/display-preferences';
import { Alert, AlertDescription } from '@/components/ui/alert';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import {
    ChartContainer,
    ChartTooltip,
    ChartTooltipContent,
    type ChartConfig,
} from '@/components/ui/chart';
import { Segmented } from '@/components/ui/segmented';
import { useT } from '@/i18n/client';
import { NUMBER_LOCALE } from '@/lib/format';
import type { Schemas } from '@/lib/types';
import { cn } from '@/lib/utils';
import { RANGES, type Range } from './ranges';

type Summary = Schemas['AIUsageSummary'];
type Group = Schemas['AIUsageGroup'];

const n = (v: number) => v.toLocaleString(NUMBER_LOCALE);
const usd = (v: string | number) => {
    const x = typeof v === 'number' ? v : Number(v);
    return `$${x.toLocaleString(NUMBER_LOCALE, {
        minimumFractionDigits: 2,
        maximumFractionDigits: x !== 0 && Math.abs(x) < 0.01 ? 6 : 2,
    })}`;
};
const ms = (v: number) =>
    v >= 1000
        ? `${(v / 1000).toLocaleString(NUMBER_LOCALE, { maximumFractionDigits: 1 })} s`
        : `${Math.round(v)} ms`;

function Kpi({
    icon: Icon,
    label,
    value,
    hint,
    tone,
    testId,
}: {
    icon: LucideIcon;
    label: string;
    value: string;
    hint?: string;
    tone?: 'warn' | 'bad';
    testId?: string;
}) {
    return (
        <Card className="gap-2 py-4" data-testid={testId}>
            <CardContent className="space-y-2 px-4">
                <div className="flex items-center justify-between gap-2">
                    <span className="text-xs font-medium text-muted-foreground">
                        {label}
                    </span>
                    <span className="flex size-7 items-center justify-center rounded-lg bg-primary/10 text-primary">
                        <Icon className="size-3.5" />
                    </span>
                </div>
                <p
                    className={cn(
                        'text-2xl font-semibold tabular-nums',
                        tone === 'warn' && 'text-amber-600 dark:text-amber-400',
                        tone === 'bad' && 'text-destructive',
                    )}
                >
                    {value}
                </p>
                {hint && (
                    <p className="text-xs text-muted-foreground">{hint}</p>
                )}
            </CardContent>
        </Card>
    );
}

/** Every day from `from` to today (UTC), with the cost of the day or 0. */
function days(from: string, groups: Group[]) {
    const byDay = new Map(groups.map((g) => [g.key, g]));
    const out: { key: string; cost: number; requests: number }[] = [];
    const d = new Date(from);
    d.setUTCHours(0, 0, 0, 0);
    const end = new Date();
    for (let i = 0; d <= end && i < 400; i++) {
        const key = d.toISOString().slice(0, 10);
        const g = byDay.get(key);
        out.push({
            key,
            cost: g ? Number(g.cost) : 0,
            requests: g?.requests ?? 0,
        });
        d.setUTCDate(d.getUTCDate() + 1);
    }
    return out;
}

function GroupTable({
    title,
    rows,
    storageKey,
    label,
    testId,
}: {
    title: string;
    rows: Group[];
    storageKey: string;
    label: (g: Group) => string;
    testId: string;
}) {
    const t = useT();
    const columns: ClientColumn<Group>[] = [
        {
            id: 'name',
            header: t('ai.usage.columns.name'),
            hideable: false,
            cell: (g) => <span className="font-medium">{label(g)}</span>,
            sortValue: (g) => label(g),
        },
        {
            id: 'requests',
            header: t('ai.usage.columns.requests'),
            cell: (g) => <span className="tabular-nums">{n(g.requests)}</span>,
            sortValue: (g) => g.requests,
        },
        {
            id: 'tokens',
            header: t('ai.usage.columns.tokens'),
            cell: (g) => (
                <span className="tabular-nums">
                    {n(g.input_tokens + g.output_tokens + g.cached_tokens)}
                </span>
            ),
            sortValue: (g) =>
                g.input_tokens + g.output_tokens + g.cached_tokens,
        },
        {
            id: 'cost',
            header: t('ai.usage.columns.cost'),
            cell: (g) => <span className="tabular-nums">{usd(g.cost)}</span>,
            sortValue: (g) => Number(g.cost),
        },
        {
            id: 'failed',
            header: t('ai.usage.columns.failed'),
            cell: (g) => <span className="tabular-nums">{n(g.failed)}</span>,
            sortValue: (g) => g.failed,
        },
        {
            id: 'latency',
            header: t('ai.usage.columns.latency'),
            cell: (g) => (
                <span className="tabular-nums">{ms(g.avg_latency_ms)}</span>
            ),
            sortValue: (g) => g.avg_latency_ms,
            defaultHidden: true,
        },
    ];
    return (
        <section className="space-y-3" data-testid={testId}>
            <h2 className="text-base font-semibold">{title}</h2>
            <ClientDataTable
                rows={rows}
                columns={columns}
                rowId={(g) => g.key || '-'}
                storageKey={storageKey}
                initialSort="-cost"
                pageSize={10}
                empty={
                    <p className="py-6 text-center text-sm text-muted-foreground">
                        {t('ai.usage.empty')}
                    </p>
                }
            />
        </section>
    );
}

/** Settings → AI → Usage: KPIs, the daily cost and the breakdowns. */
export function UsageClient({
    range,
    from,
    summary,
    byModel,
    byProvider,
    byCampaign,
    byPurpose,
}: {
    range: Range;
    from: string;
    summary: Summary;
    byModel: Group[];
    byProvider: Group[];
    byCampaign: Group[];
    byPurpose: Group[];
}) {
    const t = useT();
    const router = useRouter();
    const formatDate = useFormatDate();
    const s = summary.totals;
    const b = summary.budget;
    const daily = days(from, summary.groups).map((d) => ({
        ...d,
        label: formatDate(`${d.key}T12:00:00Z`).replace(
            /\.\d{4}$|\/\d{4}$|, \d{4}$/,
            '',
        ),
    }));
    const chart: ChartConfig = {
        cost: { label: t('ai.usage.cost'), color: 'var(--chart-1)' },
    };
    const rate = summary.handoff_rate;
    const budgetHint = b.limit
        ? t('ai.usage.budgetSpent', {
              spent: usd(b.spent),
              limit: usd(b.limit),
          })
        : t('ai.usage.budgetNone');

    return (
        <div className="space-y-6" data-testid="ai-usage">
            <div className="flex flex-wrap items-center justify-between gap-3">
                <div>
                    <h1 className="text-lg font-semibold">
                        {t('ai.usage.title')}
                    </h1>
                    <p className="text-sm text-muted-foreground">
                        {t('ai.usage.description')}
                    </p>
                </div>
                <Segmented
                    aria-label={t('ai.usage.range')}
                    value={range}
                    options={RANGES.map((r) => ({
                        value: r,
                        label: t(`ai.usage.ranges.${r}`),
                    }))}
                    onValueChange={(r) =>
                        router.push(`/settings/ai/usage?range=${r}`)
                    }
                />
            </div>

            {b.exceeded && (
                <Alert variant="destructive" data-testid="ai-budget-exceeded">
                    <AlertTriangle />
                    <AlertDescription>
                        {t('ai.usage.budgetExceeded')}
                    </AlertDescription>
                </Alert>
            )}

            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
                <Kpi
                    icon={Activity}
                    label={t('ai.usage.requests')}
                    value={n(s.requests)}
                    hint={`${t('ai.usage.replies')}: ${n(s.replies)}`}
                    testId="ai-usage-requests"
                />
                <Kpi
                    icon={MessageSquareText}
                    label={t('ai.usage.tokens')}
                    value={n(
                        s.input_tokens + s.output_tokens + s.cached_tokens,
                    )}
                    hint={t('ai.usage.tokensHint', {
                        input: n(s.input_tokens),
                        output: n(s.output_tokens),
                        cached: n(s.cached_tokens),
                    })}
                />
                <Kpi
                    icon={Coins}
                    label={t('ai.usage.cost')}
                    value={usd(s.cost)}
                    testId="ai-usage-cost"
                />
                <Kpi
                    icon={Wallet}
                    label={t('ai.usage.budget')}
                    value={usd(b.spent)}
                    hint={budgetHint}
                    tone={b.exceeded ? 'bad' : undefined}
                />
                <Kpi
                    icon={XCircle}
                    label={t('ai.usage.failed')}
                    value={n(s.failed)}
                    tone={s.failed > 0 ? 'warn' : undefined}
                />
                <Kpi
                    icon={Timer}
                    label={t('ai.usage.latency')}
                    value={ms(s.avg_latency_ms)}
                />
                <Kpi
                    icon={Repeat}
                    label={t('ai.usage.handoffRate')}
                    value={
                        rate == null
                            ? '—'
                            : `%${(rate * 100).toLocaleString(NUMBER_LOCALE, { maximumFractionDigits: 1 })}`
                    }
                    hint={t('ai.usage.handoffHint', {
                        escalations: n(summary.escalations),
                        conversations: n(summary.ai_conversations),
                    })}
                />
            </div>

            <Card>
                <CardHeader>
                    <CardTitle>{t('ai.usage.dailyCost')}</CardTitle>
                    <CardDescription>
                        {t('ai.usage.dailyCostHint')}
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    <ChartContainer
                        config={chart}
                        className="aspect-auto h-64 w-full"
                        data-testid="ai-usage-chart"
                    >
                        <BarChart data={daily} margin={{ left: -8, right: 8 }}>
                            <CartesianGrid vertical={false} />
                            <XAxis
                                dataKey="label"
                                tickLine={false}
                                axisLine={false}
                                minTickGap={24}
                            />
                            <YAxis
                                tickLine={false}
                                axisLine={false}
                                width={56}
                                tickFormatter={(v: number) => usd(v)}
                            />
                            <ChartTooltip
                                content={
                                    <ChartTooltipContent
                                        formatter={(v) => usd(Number(v))}
                                    />
                                }
                            />
                            <Bar
                                dataKey="cost"
                                fill="var(--color-cost)"
                                radius={[4, 4, 0, 0]}
                            />
                        </BarChart>
                    </ChartContainer>
                </CardContent>
            </Card>

            <div className="grid gap-6 xl:grid-cols-2">
                <GroupTable
                    title={t('ai.usage.byModel')}
                    rows={byModel}
                    storageKey="ai-usage-model"
                    label={(g) => g.label}
                    testId="ai-usage-by-model"
                />
                <GroupTable
                    title={t('ai.usage.byProvider')}
                    rows={byProvider}
                    storageKey="ai-usage-provider"
                    label={(g) => g.label}
                    testId="ai-usage-by-provider"
                />
                <GroupTable
                    title={t('ai.usage.byCampaign')}
                    rows={byCampaign}
                    storageKey="ai-usage-campaign"
                    label={(g) =>
                        g.key === '' ? t('ai.usage.outsideCampaigns') : g.label
                    }
                    testId="ai-usage-by-campaign"
                />
                <GroupTable
                    title={t('ai.usage.byPurpose')}
                    rows={byPurpose}
                    storageKey="ai-usage-purpose"
                    label={(g) =>
                        g.key in PURPOSES
                            ? t(`ai.usage.purposes.${g.key as Purpose}`)
                            : g.key
                    }
                    testId="ai-usage-by-purpose"
                />
            </div>
            <p className="text-xs text-muted-foreground">
                {t('ai.usage.otherCurrency')}
            </p>
        </div>
    );
}

const PURPOSES = {
    reply: 1,
    suggest: 1,
    classify: 1,
    extract: 1,
    summarize: 1,
    automation: 1,
    test: 1,
} as const;
type Purpose = keyof typeof PURPOSES;
