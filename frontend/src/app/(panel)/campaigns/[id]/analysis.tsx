'use client';

import { useCallback, useEffect, useRef, useState, useTransition } from 'react';
import {
    Area,
    AreaChart,
    Bar,
    BarChart,
    CartesianGrid,
    XAxis,
    YAxis,
} from 'recharts';
import { Download, FileSpreadsheet, FileText, RefreshCw } from 'lucide-react';
import { notify } from '@/components/notify';
import { useReauth } from '@/components/reauth';
import { useFormatDateTime } from '@/components/display-preferences';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import {
    ChartContainer,
    ChartLegend,
    ChartLegendContent,
    ChartTooltip,
    ChartTooltipContent,
    type ChartConfig,
} from '@/components/ui/chart';
import { Input } from '@/components/ui/input';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import {
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
    Tooltip,
    TooltipContent,
    TooltipTrigger,
} from '@/components/ui/tooltip';
import { useT } from '@/i18n/client';
import type { MessageKey, Translate } from '@/i18n/translate';
import { NUMBER_LOCALE } from '@/lib/format';
import type { Campaign } from '@/lib/types';
import {
    createExportAction,
    listExportsAction,
    loadAnalysisAction,
    loadBreakdownAction,
    setReactionMapAction,
    type BreakdownBucket,
    type CampaignAnalytics,
    type CampaignExport,
    type ForecastVsActual,
} from '../analytics-actions';

const n = (v: number | null | undefined) =>
    v == null ? '—' : v.toLocaleString(NUMBER_LOCALE);
const pct = (r: number | null | undefined) =>
    r == null
        ? '—'
        : `%${(r * 100).toLocaleString(NUMBER_LOCALE, { maximumFractionDigits: 1 })}`;
const money = (amount: string | null | undefined, currency?: string | null) =>
    amount == null || amount === ''
        ? '—'
        : `${Number(amount).toLocaleString(NUMBER_LOCALE, { maximumFractionDigits: 4 })}${currency ? ` ${currency}` : ''}`;

/** A source tag: Meta's figure or Messora's own calculation. */
export function SourceBadge({ source }: { source: 'meta' | 'system' }) {
    const t = useT();
    return (
        <Tooltip>
            <TooltipTrigger asChild>
                <Badge
                    variant={source === 'meta' ? 'secondary' : 'outline'}
                    className="text-[10px] font-normal"
                    data-source={source}
                >
                    {t(`analytics.source.${source}`)}
                </Badge>
            </TooltipTrigger>
            <TooltipContent className="max-w-64">
                {t(
                    source === 'meta'
                        ? 'analytics.source.metaHint'
                        : 'analytics.source.systemHint',
                )}
            </TooltipContent>
        </Tooltip>
    );
}

type Perms = { edit: boolean; start: boolean };

/**
 * The campaign's analysis center. Counters come live with the campaign's
 * realtime events (counts); the rest is re-read when they change, at most
 * every few seconds — no polling.
 */
export function CampaignAnalysis({
    campaign,
    counts,
    perms,
}: {
    campaign: Campaign;
    counts: Record<string, number>;
    perms: Perms;
}) {
    const t = useT();
    const [data, setData] = useState<{
        analytics: CampaignAnalytics;
        forecast: ForecastVsActual;
    } | null>(null);
    const [failed, setFailed] = useState(false);
    const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
    const load = useCallback(async () => {
        const r = await loadAnalysisAction(campaign.id);
        if (r.ok) {
            setData(r.data);
            setFailed(false);
        } else setFailed(true);
    }, [campaign.id]);
    const key = JSON.stringify(counts);
    useEffect(() => {
        clearTimeout(timer.current);
        timer.current = setTimeout(() => void load(), data ? 2500 : 0);
        return () => clearTimeout(timer.current);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [key, load]);

    return (
        <Card className="min-w-0" data-testid="analysis-center">
            <CardHeader>
                <CardTitle>{t('analytics.title')}</CardTitle>
                <CardDescription>{t('analytics.description')}</CardDescription>
            </CardHeader>
            <CardContent>
                <Tabs defaultValue="overview" className="space-y-4">
                    <TabsList className="flex h-auto flex-wrap">
                        {(
                            [
                                'overview',
                                'cost',
                                'quality',
                                'breakdowns',
                                'forecast',
                                'reports',
                            ] as const
                        ).map((k) => (
                            <TabsTrigger
                                key={k}
                                value={k}
                                data-testid={`analysis-tab-${k}`}
                            >
                                {t(`analytics.tabs.${k}`)}
                            </TabsTrigger>
                        ))}
                    </TabsList>
                    {!data ? (
                        <div className="flex items-center gap-2 py-8 text-sm text-muted-foreground">
                            {failed ? (
                                t('analytics.forecast.failed')
                            ) : (
                                <Spinner />
                            )}
                        </div>
                    ) : (
                        <>
                            <TabsContent value="overview">
                                <Overview
                                    a={data.analytics}
                                    counts={counts}
                                    campaign={campaign}
                                    canEdit={perms.edit}
                                />
                            </TabsContent>
                            <TabsContent value="cost">
                                <Cost a={data.analytics} />
                            </TabsContent>
                            <TabsContent value="quality">
                                <Quality a={data.analytics} counts={counts} />
                            </TabsContent>
                            <TabsContent value="breakdowns">
                                <Breakdowns campaignId={campaign.id} />
                            </TabsContent>
                            <TabsContent value="forecast">
                                <ForecastVsActualView f={data.forecast} />
                            </TabsContent>
                        </>
                    )}
                    <TabsContent value="reports">
                        <Reports campaignId={campaign.id} />
                    </TabsContent>
                </Tabs>
            </CardContent>
        </Card>
    );
}

function metricLabel(t: Translate, key: string): string {
    return t(`analytics.metrics.${key}` as MessageKey);
}

/** Live value of a metric: the realtime counters win over the last read. */
function liveValue(key: string, value: number, counts: Record<string, number>) {
    const map: Record<string, string> = {
        sent: 'a_sent',
        delivered: 'a_delivered',
        read: 'a_read',
        failed: 'a_failed',
        pending: 'a_pending',
        rejected: 'a_rejected',
        unknown: 'a_unknown',
        blocked: 'a_blocked',
        replied: 'a_replied',
        positive: 'a_positive',
        negative: 'a_negative',
        neutral: 'a_neutral',
        opted_out: 'a_opted_out',
        err_131048: 'a_err_131048',
        err_131049: 'a_err_131049',
        err_131050: 'a_err_131050',
    };
    const k = map[key];
    if (key === 'audience' && typeof counts.recipients === 'number')
        return counts.recipients;
    return k && typeof counts[k] === 'number' ? counts[k] : value;
}

function Overview({
    a,
    counts,
    campaign,
    canEdit,
}: {
    a: CampaignAnalytics;
    counts: Record<string, number>;
    campaign: Campaign;
    canEdit: boolean;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const shown = a.metrics.filter((m) => !m.key.startsWith('err_'));
    const hourly: ChartConfig = {
        sent: { label: t('analytics.hourly.sent'), color: 'var(--chart-1)' },
        delivered: {
            label: t('analytics.hourly.delivered'),
            color: 'var(--chart-2)',
        },
        read: { label: t('analytics.hourly.read'), color: 'var(--chart-3)' },
        replied: {
            label: t('analytics.hourly.replied'),
            color: 'var(--chart-4)',
        },
    };
    const series = a.hourly.map((p) => ({
        ...p,
        label: formatDateTime(p.hour).replace(/:\d{2}(\s|$)/, '$1'),
    }));
    return (
        <div className="space-y-6">
            <dl
                className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5"
                data-testid="analysis-metrics"
            >
                {shown.map((m) => (
                    <div
                        key={m.key}
                        className="space-y-1 rounded-lg border p-3"
                        data-metric={m.key}
                    >
                        <dt className="flex items-center justify-between gap-1 text-xs text-muted-foreground">
                            <span>{metricLabel(t, m.key)}</span>
                            <SourceBadge source={m.source} />
                        </dt>
                        <dd className="text-xl font-semibold tabular-nums">
                            {n(liveValue(m.key, m.value, counts))}
                        </dd>
                    </div>
                ))}
            </dl>
            <div className="grid gap-6 lg:grid-cols-[18rem_1fr]">
                <div className="space-y-2">
                    <p className="flex items-center gap-2 text-sm font-medium">
                        {t('analytics.rates.title')}
                        <SourceBadge source="system" />
                    </p>
                    <dl className="space-y-1 text-sm">
                        {(
                            [
                                'sent',
                                'delivery',
                                'read',
                                'reply',
                                'positive',
                                'negative',
                                'opt_out',
                            ] as const
                        ).map((k) => (
                            <div key={k} className="flex justify-between gap-2">
                                <dt className="text-muted-foreground">
                                    {t(`analytics.rates.${k}`)}
                                </dt>
                                <dd className="tabular-nums">
                                    {pct(a.rates[k])}
                                </dd>
                            </div>
                        ))}
                    </dl>
                </div>
                <div className="min-w-0 space-y-2">
                    <p className="text-sm font-medium">
                        {t('analytics.hourly.title')}
                    </p>
                    {series.length === 0 ? (
                        <p className="text-sm text-muted-foreground">
                            {t('analytics.hourly.empty')}
                        </p>
                    ) : (
                        <ChartContainer
                            config={hourly}
                            className="aspect-auto h-56 w-full"
                        >
                            <AreaChart
                                data={series}
                                margin={{ left: -16, right: 8 }}
                            >
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
                                    allowDecimals={false}
                                    width={40}
                                />
                                <ChartTooltip
                                    content={
                                        <ChartTooltipContent indicator="dot" />
                                    }
                                />
                                {(
                                    [
                                        'sent',
                                        'delivered',
                                        'read',
                                        'replied',
                                    ] as const
                                ).map((k) => (
                                    <Area
                                        key={k}
                                        dataKey={k}
                                        type="monotone"
                                        stroke={`var(--color-${k})`}
                                        fill={`var(--color-${k})`}
                                        fillOpacity={0.08}
                                        strokeWidth={2}
                                    />
                                ))}
                                <ChartLegend content={<ChartLegendContent />} />
                            </AreaChart>
                        </ChartContainer>
                    )}
                </div>
            </div>
            <ReactionMapEditor
                campaignId={campaign.id}
                buttons={a.quick_replies}
                value={
                    (campaign.reaction_map ?? {}) as Record<string, Reaction>
                }
                canEdit={canEdit}
            />
        </div>
    );
}

type Reaction = 'positive' | 'negative' | 'neutral';
const NONE = 'none';

/**
 * Quick reply button → positive / negative / neutral. Used in the wizard
 * (onChange) and on the detail page (saved with its own button).
 */
export function ReactionMapEditor({
    campaignId,
    buttons,
    value,
    canEdit,
    onChange,
}: {
    campaignId?: string;
    buttons: string[];
    value: Record<string, Reaction>;
    canEdit: boolean;
    onChange?: (v: Record<string, Reaction>) => void;
}) {
    const t = useT();
    const { guard } = useReauth();
    const [map, setMap] = useState(value);
    const [pending, startTransition] = useTransition();
    const keys = Array.from(new Set([...buttons, ...Object.keys(map)]));
    const set = (k: string, v: string) => {
        const next = { ...map };
        if (v === NONE) delete next[k];
        else next[k] = v as Reaction;
        setMap(next);
        onChange?.(next);
    };
    return (
        <div className="space-y-3" data-testid="reaction-map">
            <div>
                <p className="flex items-center gap-2 text-sm font-medium">
                    {t('analytics.reactionMap.title')}
                    <SourceBadge source="system" />
                </p>
                <p className="text-xs text-muted-foreground">
                    {t('analytics.reactionMap.description')}
                </p>
            </div>
            {keys.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                    {t('analytics.reactionMap.noButtons')}
                </p>
            ) : (
                <div className="grid gap-2 sm:grid-cols-2">
                    {keys.map((k) => (
                        <div key={k} className="flex items-center gap-2">
                            <span className="min-w-0 flex-1 truncate rounded border px-2 py-1 text-sm">
                                {k}
                            </span>
                            <Select
                                value={map[k] ?? NONE}
                                onValueChange={(v) => set(k, v)}
                                disabled={!canEdit}
                            >
                                <SelectTrigger
                                    className="w-36"
                                    aria-label={`${t('analytics.reactionMap.button')} ${k}`}
                                >
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    <SelectItem value={NONE}>
                                        {t('analytics.reactionMap.none')}
                                    </SelectItem>
                                    {(
                                        [
                                            'positive',
                                            'negative',
                                            'neutral',
                                        ] as const
                                    ).map((r) => (
                                        <SelectItem key={r} value={r}>
                                            {t(`analytics.metrics.${r}`)}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </div>
                    ))}
                </div>
            )}
            {campaignId && canEdit && keys.length > 0 && (
                <Button
                    size="sm"
                    variant="outline"
                    disabled={pending}
                    onClick={() =>
                        startTransition(async () => {
                            const r = await guard(() =>
                                setReactionMapAction(campaignId, map),
                            );
                            if (notify(r)) {
                                const { toast } = await import('sonner');
                                toast.success(t('analytics.reactionMap.saved'));
                            }
                        })
                    }
                >
                    {pending && <Spinner />}
                    {t('analytics.reactionMap.save')}
                </Button>
            )}
        </div>
    );
}

function Cost({ a }: { a: CampaignAnalytics }) {
    const t = useT();
    return (
        <div className="space-y-6">
            <div className="space-y-1 rounded-lg border p-4">
                <p className="flex items-center gap-2 text-sm text-muted-foreground">
                    {t('analytics.cost.estimate')}
                    <SourceBadge source="system" />
                </p>
                <p
                    className="text-2xl font-semibold tabular-nums"
                    data-testid="cost-estimate"
                >
                    {money(a.cost_estimate, a.currency)}
                </p>
                <p className="text-xs text-muted-foreground">
                    {t('analytics.cost.estimateHint')}{' '}
                    {t('analytics.cost.priced', { count: a.priced })}
                    {a.unpriced > 0 &&
                        ` · ${t('analytics.cost.unpriced', { count: a.unpriced })}`}
                </p>
            </div>
            <div className="grid gap-3 sm:grid-cols-2">
                <div className="space-y-1 rounded-lg border p-4">
                    <p className="flex items-center gap-2 text-sm text-muted-foreground">
                        {t('analytics.cost.ai')}
                        <SourceBadge source="system" />
                    </p>
                    <p
                        className="text-lg font-semibold tabular-nums"
                        data-testid="cost-ai"
                    >
                        {a.ai_cost.length === 0
                            ? money('0', a.currency)
                            : a.ai_cost
                                  .map((x) => money(x.cost, x.currency))
                                  .join(' + ')}
                    </p>
                    {a.ai_cost.length > 0 && (
                        <p
                            className="text-xs text-muted-foreground tabular-nums"
                            data-testid="cost-ai-usage"
                        >
                            {t('analytics.cost.aiUsage', {
                                requests: n(
                                    a.ai_cost.reduce(
                                        (s, x) => s + x.requests,
                                        0,
                                    ),
                                ),
                                tokens: n(
                                    a.ai_cost.reduce(
                                        (s, x) =>
                                            s +
                                            x.input_tokens +
                                            x.output_tokens,
                                        0,
                                    ),
                                ),
                                conversations: n(
                                    a.ai_cost.reduce(
                                        (s, x) => s + x.conversations,
                                        0,
                                    ),
                                ),
                            })}
                        </p>
                    )}
                    <p className="text-xs text-muted-foreground">
                        {t('analytics.cost.aiHint')}
                    </p>
                </div>
                <div className="space-y-1 rounded-lg border p-4">
                    <p className="flex items-center gap-2 text-sm text-muted-foreground">
                        {t('analytics.cost.total')}
                        <SourceBadge source="system" />
                    </p>
                    <p
                        className="text-lg font-semibold tabular-nums"
                        data-testid="cost-total"
                    >
                        {money(a.total_cost, a.currency)}
                    </p>
                </div>
            </div>
            <div className="space-y-2">
                <p className="text-sm font-medium">
                    {t('analytics.cost.byCategory')}
                </p>
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>
                                {t('analytics.cost.category')}
                            </TableHead>
                            <TableHead>{t('analytics.cost.type')}</TableHead>
                            <TableHead className="text-right">
                                {t('analytics.cost.messages')}
                            </TableHead>
                            <TableHead className="text-right">
                                {t('analytics.cost.amount')}
                            </TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {a.cost_breakdown.map((c) => (
                            <TableRow key={`${c.category}-${c.type}`}>
                                <TableCell>{c.category || '—'}</TableCell>
                                <TableCell>
                                    <span className="inline-flex items-center gap-2">
                                        {c.type || '—'}{' '}
                                        <SourceBadge source="meta" />
                                    </span>
                                </TableCell>
                                <TableCell className="text-right tabular-nums">
                                    {n(c.messages)}
                                </TableCell>
                                <TableCell className="text-right tabular-nums">
                                    {money(c.cost, c.currency)}
                                </TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            </div>
            <div className="space-y-2">
                <p className="flex items-center gap-2 text-sm font-medium">
                    {t('analytics.cost.reconciled')}{' '}
                    <SourceBadge source="meta" />
                </p>
                <p className="text-xs text-muted-foreground">
                    {t('analytics.cost.reconciledHint')}
                </p>
                {a.reconciled.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                        {t('analytics.cost.noReconciled')}
                    </p>
                ) : (
                    <Table>
                        <TableHeader>
                            <TableRow>
                                <TableHead>{t('analytics.cost.day')}</TableHead>
                                <TableHead>
                                    {t('analytics.cost.category')}
                                </TableHead>
                                <TableHead className="text-right">
                                    {t('analytics.cost.volume')}
                                </TableHead>
                                <TableHead className="text-right">
                                    {t('analytics.cost.amount')}
                                </TableHead>
                            </TableRow>
                        </TableHeader>
                        <TableBody>
                            {a.reconciled.map((d) => (
                                <TableRow key={`${d.day}-${d.category}`}>
                                    <TableCell>{d.day}</TableCell>
                                    <TableCell>{d.category || '—'}</TableCell>
                                    <TableCell className="text-right tabular-nums">
                                        {n(d.volume)}
                                    </TableCell>
                                    <TableCell className="text-right tabular-nums">
                                        {money(d.cost, d.currency)}
                                    </TableCell>
                                </TableRow>
                            ))}
                        </TableBody>
                    </Table>
                )}
            </div>
        </div>
    );
}

function eventText(t: Translate, kind: string, p: Record<string, unknown>) {
    const s = (v: unknown) => (v == null || v === '' ? '—' : String(v));
    if (kind === 'quality_rating_change')
        return t('analytics.quality.kinds.quality_rating_change', {
            from: s(p.from),
            to: s(p.to),
        });
    if (kind === 'template_change')
        return t('analytics.quality.kinds.template_change', {
            template: s(p.template),
            statusFrom: s(p.status_from),
            statusTo: s(p.status_to),
            qualityFrom: s(p.quality_from),
            qualityTo: s(p.quality_to),
        });
    if (kind === 'message_template_quality_update')
        return t('analytics.quality.kinds.message_template_quality_update');
    return t('analytics.quality.kinds.other', {
        kind: kind.replaceAll('_', ' '),
    });
}

function Quality({
    a,
    counts,
}: {
    a: CampaignAnalytics;
    counts: Record<string, number>;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const delivered = counts.a_delivered ?? 0;
    const optOut = delivered > 0 ? (counts.a_opted_out ?? 0) / delivered : null;
    return (
        <div className="space-y-6">
            <Alert>
                <AlertDescription>
                    {t('analytics.quality.noSpamReports')}
                </AlertDescription>
            </Alert>
            <dl className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6">
                {(
                    [
                        ['numberQuality', a.number_quality || '—', 'meta'],
                        ['templateStatus', a.template_status || '—', 'meta'],
                        ['templateQuality', a.template_quality || '—', 'meta'],
                    ] as const
                ).map(([k, v, src]) => (
                    <div key={k} className="space-y-1 rounded-lg border p-3">
                        <dt className="flex items-center justify-between gap-1 text-xs text-muted-foreground">
                            {t(`analytics.quality.${k}`)}{' '}
                            <SourceBadge source={src} />
                        </dt>
                        <dd className="font-semibold">{v}</dd>
                    </div>
                ))}
                {(['err_131048', 'err_131049', 'err_131050'] as const).map(
                    (k) => (
                        <div
                            key={k}
                            className="space-y-1 rounded-lg border p-3"
                            data-metric={k}
                        >
                            <dt className="flex items-center justify-between gap-1 text-xs text-muted-foreground">
                                {t(`analytics.metrics.${k}`)}{' '}
                                <SourceBadge source="meta" />
                            </dt>
                            <dd className="font-semibold tabular-nums">
                                {n(counts[`a_${k}`] ?? 0)}
                            </dd>
                        </div>
                    ),
                )}
                <div className="space-y-1 rounded-lg border p-3">
                    <dt className="flex items-center justify-between gap-1 text-xs text-muted-foreground">
                        {t('analytics.quality.optOutRate')}{' '}
                        <SourceBadge source="system" />
                    </dt>
                    <dd className="font-semibold tabular-nums">
                        {pct(optOut)}
                    </dd>
                </div>
            </dl>
            <div className="space-y-2">
                <p className="text-sm font-medium">
                    {t('analytics.quality.timeline')}
                </p>
                {a.timeline.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                        {t('analytics.quality.noEvents')}
                    </p>
                ) : (
                    <ol className="space-y-1 text-sm">
                        {a.timeline.map((e, i) => (
                            <li key={i} className="flex gap-3">
                                <span className="w-40 shrink-0 text-muted-foreground tabular-nums">
                                    {formatDateTime(e.occurred_at)}
                                </span>
                                <span>{eventText(t, e.kind, e.payload)}</span>
                            </li>
                        ))}
                    </ol>
                )}
            </div>
        </div>
    );
}

const DIMENSIONS = [
    'group',
    'segment',
    'tag',
    'contact_type',
    'attribute',
] as const;
type Dimension = (typeof DIMENSIONS)[number];

function Breakdowns({ campaignId }: { campaignId: string }) {
    const t = useT();
    const [dim, setDim] = useState<Dimension>('tag');
    const [attrKey, setAttrKey] = useState('');
    const [rows, setRows] = useState<BreakdownBucket[] | null>(null);
    const [pending, startTransition] = useTransition();
    const load = (d: Dimension, key: string) =>
        startTransition(async () => {
            if (d === 'attribute' && !key.trim()) {
                setRows([]);
                return;
            }
            const r = await loadBreakdownAction(campaignId, d, key);
            if (notify(r)) setRows(r.data);
        });
    useEffect(() => {
        load('tag', '');
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [campaignId]);
    const chart: ChartConfig = {
        delivered: {
            label: t('analytics.breakdown.delivered'),
            color: 'var(--chart-2)',
        },
        read: { label: t('analytics.breakdown.read'), color: 'var(--chart-3)' },
        replied: {
            label: t('analytics.breakdown.replied'),
            color: 'var(--chart-4)',
        },
    };
    const label = (b: BreakdownBucket) =>
        b.label || t('analytics.breakdown.none');
    const data = (rows ?? []).slice(0, 12).map((b) => ({
        name: label(b).length > 16 ? `${label(b).slice(0, 15)}…` : label(b),
        delivered: b.delivered,
        read: b.read,
        replied: b.replied,
    }));
    return (
        <div className="space-y-4" data-testid="breakdowns">
            <div className="flex flex-wrap items-end gap-2">
                <Tabs
                    value={dim}
                    onValueChange={(v) => {
                        setDim(v as Dimension);
                        load(v as Dimension, attrKey);
                    }}
                >
                    <TabsList>
                        {DIMENSIONS.map((d) => (
                            <TabsTrigger key={d} value={d}>
                                {t(`analytics.breakdown.${d}`)}
                            </TabsTrigger>
                        ))}
                    </TabsList>
                </Tabs>
                {dim === 'attribute' && (
                    <form
                        className="flex gap-2"
                        onSubmit={(e) => {
                            e.preventDefault();
                            load('attribute', attrKey);
                        }}
                    >
                        <Input
                            value={attrKey}
                            onChange={(e) => setAttrKey(e.target.value)}
                            placeholder={t('analytics.breakdown.attributeKey')}
                            className="w-48"
                            maxLength={100}
                        />
                        <Button type="submit" variant="outline">
                            {t('analytics.breakdown.show')}
                        </Button>
                    </form>
                )}
                {pending && <Spinner />}
            </div>
            {rows && rows.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                    {t('analytics.breakdown.empty')}
                </p>
            ) : (
                rows && (
                    <>
                        <ChartContainer
                            config={chart}
                            className="aspect-auto h-56 w-full"
                        >
                            <BarChart
                                data={data}
                                margin={{ left: 0, right: 8 }}
                            >
                                <CartesianGrid vertical={false} />
                                <XAxis
                                    dataKey="name"
                                    tickLine={false}
                                    axisLine={false}
                                    interval={0}
                                    fontSize={11}
                                />
                                <YAxis
                                    tickLine={false}
                                    axisLine={false}
                                    allowDecimals={false}
                                    width={40}
                                />
                                <ChartTooltip
                                    content={<ChartTooltipContent />}
                                />
                                <ChartLegend content={<ChartLegendContent />} />
                                {(
                                    ['delivered', 'read', 'replied'] as const
                                ).map((k) => (
                                    <Bar
                                        key={k}
                                        dataKey={k}
                                        maxBarSize={40}
                                        fill={`var(--color-${k})`}
                                        radius={[4, 4, 0, 0]}
                                    />
                                ))}
                            </BarChart>
                        </ChartContainer>
                        <Table>
                            <TableHeader>
                                <TableRow>
                                    <TableHead>
                                        {t('analytics.breakdown.name')}
                                    </TableHead>
                                    {(
                                        [
                                            'recipients',
                                            'sent',
                                            'delivered',
                                            'read',
                                            'replied',
                                            'positive',
                                            'negative',
                                            'opted_out',
                                            'cost',
                                        ] as const
                                    ).map((k) => (
                                        <TableHead
                                            key={k}
                                            className="text-right"
                                        >
                                            {t(`analytics.breakdown.${k}`)}
                                        </TableHead>
                                    ))}
                                </TableRow>
                            </TableHeader>
                            <TableBody>
                                {rows.map((b) => (
                                    <TableRow key={b.key} data-bucket={b.label}>
                                        <TableCell>{label(b)}</TableCell>
                                        <TableCell className="text-right tabular-nums">
                                            {n(b.recipients)}
                                        </TableCell>
                                        <TableCell className="text-right tabular-nums">
                                            {n(b.sent)}
                                        </TableCell>
                                        <TableCell className="text-right tabular-nums">
                                            {n(b.delivered)}
                                        </TableCell>
                                        <TableCell className="text-right tabular-nums">
                                            {n(b.read)}
                                        </TableCell>
                                        <TableCell className="text-right tabular-nums">
                                            {n(b.replied)}
                                        </TableCell>
                                        <TableCell className="text-right tabular-nums">
                                            {n(b.positive)}
                                        </TableCell>
                                        <TableCell className="text-right tabular-nums">
                                            {n(b.negative)}
                                        </TableCell>
                                        <TableCell className="text-right tabular-nums">
                                            {n(b.opted_out)}
                                        </TableCell>
                                        <TableCell className="text-right tabular-nums">
                                            {money(b.cost)}
                                        </TableCell>
                                    </TableRow>
                                ))}
                            </TableBody>
                        </Table>
                    </>
                )
            )}
        </div>
    );
}

const RATE_METRICS = new Set([
    'sent_rate',
    'delivery_rate',
    'read_rate',
    'reply_rate',
    'positive_rate',
    'opt_out_rate',
]);

function fmtMetric(metric: string, v: number | null | undefined): string {
    if (v == null) return '—';
    if (RATE_METRICS.has(metric)) return pct(v);
    if (metric === 'duration_seconds') {
        const h = Math.floor(v / 3600);
        const m = Math.round((v % 3600) / 60);
        return h > 0 ? `${h} sa ${m} dk` : `${m} dk`;
    }
    if (metric === 'cost')
        return v.toLocaleString(NUMBER_LOCALE, { maximumFractionDigits: 4 });
    return n(v);
}

export function ForecastVsActualView({ f }: { f: ForecastVsActual }) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    if (!f.forecast) {
        return (
            <p className="text-sm text-muted-foreground">
                {t('analytics.fva.none')}
            </p>
        );
    }
    return (
        <div className="space-y-3" data-testid="forecast-vs-actual">
            <p className="text-sm text-muted-foreground">
                {f.final ? t('analytics.fva.final') : t('analytics.fva.live')}{' '}
                {f.stored_at &&
                    t('analytics.fva.storedAt', {
                        date: formatDateTime(f.stored_at),
                    })}
            </p>
            {f.forecast.rates.status !== 'ok' && (
                <p className="text-sm text-muted-foreground">
                    {t('analytics.fva.insufficient')}
                </p>
            )}
            <Table>
                <TableHeader>
                    <TableRow>
                        <TableHead>{t('analytics.fva.metric')}</TableHead>
                        <TableHead className="text-right">
                            {t('analytics.fva.forecast')}
                        </TableHead>
                        <TableHead className="text-right">
                            {t('analytics.fva.actual')}
                        </TableHead>
                        <TableHead className="text-right">
                            {t('analytics.fva.delta')}
                        </TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {f.metrics.map((m) => (
                        <TableRow key={m.metric} data-metric={m.metric}>
                            <TableCell>
                                {t(
                                    `analytics.fva.names.${m.metric}` as MessageKey,
                                )}
                            </TableCell>
                            <TableCell className="text-right tabular-nums">
                                {fmtMetric(m.metric, m.forecast)}
                            </TableCell>
                            <TableCell className="text-right tabular-nums">
                                {fmtMetric(m.metric, m.actual)}
                            </TableCell>
                            <TableCell className="text-right tabular-nums">
                                {m.delta == null
                                    ? '—'
                                    : `${m.delta > 0 ? '+' : ''}${fmtMetric(m.metric, m.delta)}${m.delta_pct != null ? ` (${m.delta_pct > 0 ? '+' : ''}${m.delta_pct.toLocaleString(NUMBER_LOCALE)}%)` : ''}`}
                            </TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>
        </div>
    );
}

function Reports({ campaignId }: { campaignId: string }) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const { guard } = useReauth();
    const [list, setList] = useState<CampaignExport[] | null>(null);
    const [pending, startTransition] = useTransition();
    const refresh = useCallback(async () => {
        const r = await listExportsAction(campaignId);
        if (r.ok) setList(r.data);
    }, [campaignId]);
    useEffect(() => {
        let cancelled = false;
        listExportsAction(campaignId).then((r) => {
            if (!cancelled && r.ok) setList(r.data);
        });
        return () => {
            cancelled = true;
        };
    }, [campaignId]);
    // While something is being prepared, look again now and then.
    const busy = list?.some(
        (e) => e.status === 'queued' || e.status === 'running',
    );
    useEffect(() => {
        if (!busy) return;
        const id = setTimeout(() => void refresh(), 3000);
        return () => clearTimeout(id);
    }, [busy, list, refresh]);
    const request = (kind: 'recipients' | 'report', format: 'xlsx' | 'pdf') =>
        startTransition(async () => {
            const r = await guard(() =>
                createExportAction(campaignId, { kind, format }),
            );
            if (notify(r)) {
                const { toast } = await import('sonner');
                toast.success(t('analytics.reports.queued'));
                await refresh();
            }
        });
    return (
        <div className="space-y-4" data-testid="reports">
            <p className="text-sm text-muted-foreground">
                {t('analytics.reports.description')}
            </p>
            <div className="flex flex-wrap gap-2">
                <Button
                    variant="outline"
                    disabled={pending}
                    onClick={() => request('report', 'xlsx')}
                >
                    <FileSpreadsheet /> {t('analytics.reports.xlsx')}
                </Button>
                <Button
                    variant="outline"
                    disabled={pending}
                    onClick={() => request('report', 'pdf')}
                >
                    <FileText /> {t('analytics.reports.pdf')}
                </Button>
                <Button
                    variant="outline"
                    disabled={pending}
                    onClick={() => request('recipients', 'xlsx')}
                >
                    <FileSpreadsheet /> {t('analytics.reports.recipientsXlsx')}
                </Button>
                <Button
                    variant="ghost"
                    onClick={() => void refresh()}
                    aria-label={t('analytics.reports.refresh')}
                >
                    <RefreshCw />
                </Button>
            </div>
            {list && list.length === 0 && (
                <p className="text-sm text-muted-foreground">
                    {t('analytics.reports.empty')}
                </p>
            )}
            {list && list.length > 0 && (
                <Table>
                    <TableBody>
                        {list.map((e) => (
                            <TableRow key={e.id} data-export-status={e.status}>
                                <TableCell>
                                    {t(`analytics.reports.kind.${e.kind}`)} ·{' '}
                                    {e.format.toUpperCase()}
                                </TableCell>
                                <TableCell className="text-muted-foreground">
                                    {formatDateTime(e.created_at)}
                                </TableCell>
                                <TableCell>
                                    <Badge variant="outline">
                                        {t(
                                            `analytics.reports.status.${e.status}`,
                                        )}
                                    </Badge>
                                    {e.error && (
                                        <span className="ml-2 text-xs text-destructive">
                                            {e.error}
                                        </span>
                                    )}
                                </TableCell>
                                <TableCell className="text-right">
                                    {e.status === 'done' && (
                                        <Button
                                            asChild
                                            size="sm"
                                            variant="outline"
                                        >
                                            <a
                                                href={`/bff/campaigns/${campaignId}/exports/${e.id}`}
                                                download={
                                                    e.filename ?? undefined
                                                }
                                            >
                                                <Download />{' '}
                                                {t(
                                                    'analytics.reports.download',
                                                )}
                                            </a>
                                        </Button>
                                    )}
                                </TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            )}
        </div>
    );
}
