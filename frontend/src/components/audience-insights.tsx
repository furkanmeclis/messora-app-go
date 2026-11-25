'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
    Area,
    AreaChart,
    Bar,
    BarChart,
    CartesianGrid,
    XAxis,
    YAxis,
} from 'recharts';
import {
    Activity,
    CheckCheck,
    Eye,
    Megaphone,
    MessageCircleReply,
    Send,
    ShieldCheck,
    Sparkles,
    Users,
} from 'lucide-react';
import { ClientDataTable } from '@/components/client-data-table';
import { useFormatDate } from '@/components/display-preferences';
import { Badge } from '@/components/ui/badge';
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
import {
    Tooltip,
    TooltipContent,
    TooltipTrigger,
} from '@/components/ui/tooltip';
import { useT } from '@/i18n/client';
import { NUMBER_LOCALE } from '@/lib/format';
import type { Schemas } from '@/lib/types';
import { cn } from '@/lib/utils';

type Insights = Schemas['AudienceInsights'];

const n = (v: number) => v.toLocaleString(NUMBER_LOCALE);
const rate = (part: number, whole: number) => (whole > 0 ? part / whole : null);
const pct = (r: number | null) =>
    r == null
        ? '—'
        : `%${(r * 100).toLocaleString(NUMBER_LOCALE, { maximumFractionDigits: r < 0.1 ? 1 : 0 })}`;

function Kpi({
    icon: Icon,
    label,
    value,
    hint,
    tone,
    progress,
}: {
    icon: typeof Users;
    label: string;
    value: string;
    hint?: string;
    tone?: 'good' | 'warn' | 'bad';
    progress?: number | null;
}) {
    return (
        <Card className="gap-2 py-4">
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
                        tone === 'good' &&
                            'text-emerald-600 dark:text-emerald-400',
                        tone === 'warn' && 'text-amber-600 dark:text-amber-400',
                        tone === 'bad' && 'text-destructive',
                    )}
                >
                    {value}
                </p>
                {progress != null && (
                    <div className="h-1.5 overflow-hidden rounded-full bg-muted">
                        <div
                            className="h-full rounded-full bg-primary transition-all"
                            style={{
                                width: `${Math.min(100, Math.round(progress * 100))}%`,
                            }}
                        />
                    </div>
                )}
                {hint && (
                    <p className="text-xs text-muted-foreground">{hint}</p>
                )}
            </CardContent>
        </Card>
    );
}

function toneOf(r: number | null, good: number, warn: number) {
    if (r == null) return undefined;
    return r >= good ? 'good' : r >= warn ? 'warn' : 'bad';
}

/**
 * Insights of a group, tag or segment: who is in it, how messages to it
 * perform, and every campaign that reached any of its contacts.
 */
export function AudienceInsightsView({
    data,
    contactsHref,
}: {
    data: Insights;
    contactsHref?: string;
}) {
    const t = useT();
    const router = useRouter();
    const formatDate = useFormatDate();
    const { summary: s, messaging: m } = data;
    const delivery = rate(m.delivered, m.sent);
    const read = rate(m.read, m.delivered);
    const reply = rate(m.contacts_replied, m.contacts_messaged);

    const activity: ChartConfig = {
        sent: { label: t('insights.activity.sent'), color: 'var(--chart-1)' },
        read: { label: t('insights.activity.read'), color: 'var(--chart-2)' },
        inbound: {
            label: t('insights.activity.inbound'),
            color: 'var(--chart-4)',
        },
    };
    const daily = data.daily.map((d) => ({
        ...d,
        label: formatDate(`${d.day}T12:00:00Z`).replace(
            /\.\d{4}$|\/\d{4}$|, \d{4}$/,
            '',
        ),
    }));
    const compare: ChartConfig = {
        read: { label: t('insights.campaigns.read'), color: 'var(--chart-2)' },
        replied: {
            label: t('insights.campaigns.replied'),
            color: 'var(--chart-3)',
        },
    };
    const byCampaign = data.campaigns
        .filter((c) => c.sent > 0)
        .slice(0, 8)
        .reverse()
        .map((c) => ({
            name: c.name.length > 18 ? `${c.name.slice(0, 17)}…` : c.name,
            read: Math.round((rate(c.read, c.delivered) ?? 0) * 100),
            replied: Math.round((rate(c.replied, c.sent) ?? 0) * 100),
        }));
    const funnel = [
        { key: 'sent', label: t('insights.funnel.sent'), value: m.sent },
        {
            key: 'delivered',
            label: t('insights.funnel.delivered'),
            value: m.delivered,
        },
        { key: 'read', label: t('insights.funnel.read'), value: m.read },
        {
            key: 'replied',
            label: t('insights.funnel.replied'),
            value: m.contacts_replied,
        },
    ];
    const top = Math.max(1, m.sent);

    return (
        <div className="space-y-6" data-testid="audience-insights">
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3 2xl:grid-cols-6">
                <Kpi
                    icon={Users}
                    label={t('insights.kpi.contacts')}
                    value={n(s.contacts)}
                    hint={t('insights.kpi.newThisMonth', {
                        count: n(s.new_30d),
                    })}
                />
                <Kpi
                    icon={ShieldCheck}
                    label={t('insights.kpi.reachable')}
                    value={n(s.reachable)}
                    progress={rate(s.reachable, s.contacts)}
                    hint={t('insights.kpi.optedOut', { count: n(s.opted_out) })}
                />
                <Kpi
                    icon={Activity}
                    label={t('insights.kpi.active')}
                    value={n(s.active_30d)}
                    progress={rate(s.active_30d, s.contacts)}
                    hint={t('insights.kpi.active7', { count: n(s.active_7d) })}
                />
                <Kpi
                    icon={CheckCheck}
                    label={t('insights.kpi.delivery')}
                    value={pct(delivery)}
                    tone={toneOf(delivery, 0.9, 0.7)}
                    progress={delivery}
                    hint={t('insights.kpi.ofSent', { count: n(m.sent) })}
                />
                <Kpi
                    icon={Eye}
                    label={t('insights.kpi.readRate')}
                    value={pct(read)}
                    tone={toneOf(read, 0.6, 0.3)}
                    progress={read}
                    hint={t('insights.kpi.ofDelivered', {
                        count: n(m.delivered),
                    })}
                />
                <Kpi
                    icon={MessageCircleReply}
                    label={t('insights.kpi.replyRate')}
                    value={pct(reply)}
                    tone={toneOf(reply, 0.2, 0.05)}
                    progress={reply}
                    hint={t('insights.kpi.ofMessaged', {
                        count: n(m.contacts_replied),
                        total: n(m.contacts_messaged),
                    })}
                />
            </div>

            <div className="grid gap-4 xl:grid-cols-[1fr_22rem]">
                <Card>
                    <CardHeader>
                        <CardTitle>{t('insights.activity.title')}</CardTitle>
                        <CardDescription>
                            {t('insights.activity.description')}
                        </CardDescription>
                    </CardHeader>
                    <CardContent>
                        <ChartContainer
                            config={activity}
                            className="aspect-auto h-64 w-full"
                        >
                            <AreaChart
                                data={daily}
                                margin={{ left: -16, right: 8 }}
                            >
                                <defs>
                                    {(['sent', 'read', 'inbound'] as const).map(
                                        (k) => (
                                            <linearGradient
                                                key={k}
                                                id={`fill-${k}`}
                                                x1="0"
                                                y1="0"
                                                x2="0"
                                                y2="1"
                                            >
                                                <stop
                                                    offset="5%"
                                                    stopColor={`var(--color-${k})`}
                                                    stopOpacity={0.35}
                                                />
                                                <stop
                                                    offset="95%"
                                                    stopColor={`var(--color-${k})`}
                                                    stopOpacity={0.02}
                                                />
                                            </linearGradient>
                                        ),
                                    )}
                                </defs>
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
                                {(['sent', 'read', 'inbound'] as const).map(
                                    (k) => (
                                        <Area
                                            key={k}
                                            dataKey={k}
                                            type="monotone"
                                            stroke={`var(--color-${k})`}
                                            strokeWidth={2}
                                            fill={`url(#fill-${k})`}
                                        />
                                    ),
                                )}
                                <ChartLegend content={<ChartLegendContent />} />
                            </AreaChart>
                        </ChartContainer>
                    </CardContent>
                </Card>
                <Card>
                    <CardHeader>
                        <CardTitle>{t('insights.funnel.title')}</CardTitle>
                        <CardDescription>
                            {t('insights.funnel.description')}
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="space-y-3">
                        {funnel.map((f, i) => (
                            <div key={f.key} className="space-y-1">
                                <div className="flex items-baseline justify-between gap-2 text-sm">
                                    <span>{f.label}</span>
                                    <span className="font-medium tabular-nums">
                                        {n(f.value)}
                                        {i > 0 && (
                                            <span className="ml-1.5 text-xs font-normal text-muted-foreground">
                                                {pct(
                                                    rate(
                                                        f.value,
                                                        funnel[i - 1]!.value,
                                                    ),
                                                )}
                                            </span>
                                        )}
                                    </span>
                                </div>
                                <div className="h-2.5 overflow-hidden rounded-full bg-muted">
                                    <div
                                        className="h-full rounded-full"
                                        style={{
                                            width: `${Math.max(f.value > 0 ? 2 : 0, (f.value / top) * 100)}%`,
                                            background: `var(--chart-${i + 1})`,
                                        }}
                                    />
                                </div>
                            </div>
                        ))}
                        <div className="flex flex-wrap gap-x-3 gap-y-1 pt-1 text-xs text-muted-foreground">
                            <span>
                                {t('insights.funnel.failed', {
                                    count: n(m.failed),
                                })}
                            </span>
                            <span>
                                {t('insights.funnel.fromCampaigns', {
                                    count: n(m.from_campaigns),
                                })}
                            </span>
                        </div>
                    </CardContent>
                </Card>
            </div>

            <Card>
                <CardHeader className="flex flex-row flex-wrap items-start justify-between gap-2">
                    <div className="space-y-1.5">
                        <CardTitle>{t('insights.campaigns.title')}</CardTitle>
                        <CardDescription>
                            {t('insights.campaigns.description')}
                        </CardDescription>
                    </div>
                    {contactsHref && (
                        <Link
                            href={contactsHref}
                            className="text-sm text-primary hover:underline"
                        >
                            {t('insights.viewContacts')}
                        </Link>
                    )}
                </CardHeader>
                <CardContent className="space-y-6">
                    {data.campaigns.length === 0 ? (
                        <p className="rounded-lg border border-dashed p-8 text-center text-sm text-muted-foreground">
                            {t('insights.campaigns.empty')}
                        </p>
                    ) : (
                        <>
                            {byCampaign.length > 1 && (
                                <div className="space-y-2">
                                    <p className="text-sm font-medium">
                                        {t('insights.campaigns.compare')}
                                    </p>
                                    <ChartContainer
                                        config={compare}
                                        className="aspect-auto h-56 w-full"
                                    >
                                        <BarChart
                                            data={byCampaign}
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
                                                unit="%"
                                                width={52}
                                                domain={[0, 100]}
                                            />
                                            <ChartTooltip
                                                content={
                                                    <ChartTooltipContent />
                                                }
                                            />
                                            <ChartLegend
                                                content={<ChartLegendContent />}
                                            />
                                            <Bar
                                                dataKey="read"
                                                maxBarSize={56}
                                                fill="var(--color-read)"
                                                radius={[4, 4, 0, 0]}
                                            />
                                            <Bar
                                                dataKey="replied"
                                                maxBarSize={56}
                                                fill="var(--color-replied)"
                                                radius={[4, 4, 0, 0]}
                                            />
                                        </BarChart>
                                    </ChartContainer>
                                </div>
                            )}
                            <ClientDataTable
                                rows={data.campaigns}
                                rowId={(c) => c.id}
                                storageKey="insight-campaigns"
                                initialSort="-started"
                                pageSize={10}
                                rowTestId="insight-campaign"
                                searchText={(c) =>
                                    `${c.name} ${c.template_name}`
                                }
                                searchPlaceholder={t('table.search')}
                                noMatch={
                                    <p className="p-8 text-center text-sm text-muted-foreground">
                                        {t('table.noMatch')}
                                    </p>
                                }
                                columns={[
                                    {
                                        id: 'name',
                                        header: t('insights.campaigns.name'),
                                        hideable: false,
                                        className: 'min-w-56',
                                        sortValue: (c) => c.name,
                                        cell: (c) => (
                                            <>
                                                <div className="flex flex-wrap items-center gap-2">
                                                    <Link
                                                        href={`/campaigns/${c.id}`}
                                                        className="font-medium hover:underline"
                                                    >
                                                        {c.name}
                                                    </Link>
                                                    <Tooltip>
                                                        <TooltipTrigger asChild>
                                                            <Badge
                                                                variant={
                                                                    c.targeted
                                                                        ? 'default'
                                                                        : 'secondary'
                                                                }
                                                            >
                                                                {c.targeted
                                                                    ? t(
                                                                          'insights.campaigns.targeted',
                                                                      )
                                                                    : t(
                                                                          'insights.campaigns.indirect',
                                                                      )}
                                                            </Badge>
                                                        </TooltipTrigger>
                                                        <TooltipContent>
                                                            {c.targeted
                                                                ? t(
                                                                      'insights.campaigns.targetedHint',
                                                                  )
                                                                : t(
                                                                      'insights.campaigns.indirectHint',
                                                                  )}
                                                        </TooltipContent>
                                                    </Tooltip>
                                                </div>
                                                <p className="font-mono text-xs text-muted-foreground">
                                                    {c.template_name}
                                                </p>
                                            </>
                                        ),
                                    },
                                    {
                                        id: 'reach',
                                        header: t('insights.campaigns.reach'),
                                        className: 'text-right',
                                        sortValue: (c) => c.reached,
                                        cell: (c) => (
                                            <span className="whitespace-nowrap tabular-nums">
                                                {t(
                                                    'insights.campaigns.reachOf',
                                                    {
                                                        reached: n(c.reached),
                                                        total: n(
                                                            c.total_recipients,
                                                        ),
                                                    },
                                                )}
                                            </span>
                                        ),
                                    },
                                    {
                                        id: 'delivered',
                                        header: t(
                                            'insights.campaigns.delivered',
                                        ),
                                        className: 'text-right',
                                        sortValue: (c) =>
                                            rate(c.delivered, c.sent),
                                        cell: (c) => (
                                            <RateCell
                                                part={c.delivered}
                                                whole={c.sent}
                                            />
                                        ),
                                    },
                                    {
                                        id: 'read',
                                        header: t('insights.campaigns.read'),
                                        className: 'text-right',
                                        sortValue: (c) =>
                                            rate(c.read, c.delivered),
                                        cell: (c) => (
                                            <RateCell
                                                part={c.read}
                                                whole={c.delivered}
                                            />
                                        ),
                                    },
                                    {
                                        id: 'replied',
                                        header: t('insights.campaigns.replied'),
                                        className: 'text-right',
                                        sortValue: (c) =>
                                            rate(c.replied, c.sent),
                                        cell: (c) => (
                                            <RateCell
                                                part={c.replied}
                                                whole={c.sent}
                                            />
                                        ),
                                    },
                                    {
                                        id: 'failed',
                                        header: t('insights.campaigns.failed'),
                                        className: 'text-right',
                                        sortValue: (c) => c.failed,
                                        cell: (c) => (
                                            <span
                                                className={cn(
                                                    'tabular-nums',
                                                    c.failed > 0 &&
                                                        'text-destructive',
                                                )}
                                            >
                                                {n(c.failed)}
                                            </span>
                                        ),
                                    },
                                    {
                                        id: 'optOuts',
                                        header: t('insights.campaigns.optOuts'),
                                        className: 'text-right',
                                        sortValue: (c) => c.opted_out_after,
                                        cell: (c) => (
                                            <span
                                                className={cn(
                                                    'tabular-nums',
                                                    c.opted_out_after > 0 &&
                                                        'text-amber-600',
                                                )}
                                            >
                                                {n(c.opted_out_after)}
                                            </span>
                                        ),
                                    },
                                    {
                                        id: 'started',
                                        header: t('insights.campaigns.started'),
                                        sortValue: (c) => c.started_at,
                                        cell: (c) => (
                                            <span className="text-sm whitespace-nowrap text-muted-foreground">
                                                {c.started_at
                                                    ? formatDate(c.started_at)
                                                    : '—'}
                                            </span>
                                        ),
                                    },
                                ]}
                                rowMenu={(c) => [
                                    [
                                        {
                                            id: 'open',
                                            label: c.name,
                                            icon: Megaphone,
                                            run: () =>
                                                router.push(
                                                    `/campaigns/${c.id}`,
                                                ),
                                        },
                                    ],
                                ]}
                                onRowClick={(c) =>
                                    router.push(`/campaigns/${c.id}`)
                                }
                            />
                        </>
                    )}
                </CardContent>
            </Card>

            <Card>
                <CardHeader>
                    <CardTitle className="flex items-center gap-2">
                        <Sparkles className="size-4 text-primary" />{' '}
                        {t('insights.engaged.title')}
                    </CardTitle>
                    <CardDescription>
                        {t('insights.engaged.description')}
                    </CardDescription>
                </CardHeader>
                <CardContent>
                    {data.engaged.length === 0 ? (
                        <p className="text-sm text-muted-foreground">
                            {t('insights.engaged.empty')}
                        </p>
                    ) : (
                        <ul className="grid gap-2 sm:grid-cols-2 xl:grid-cols-5">
                            {data.engaged.map((c) => (
                                <li key={c.id}>
                                    <Link
                                        href={`/contacts/${c.id}`}
                                        className="flex flex-col gap-0.5 rounded-lg border p-3 hover:bg-muted/50"
                                    >
                                        <span className="truncate font-medium">
                                            {c.name || c.phone}
                                        </span>
                                        <span className="text-xs text-muted-foreground">
                                            {c.phone}
                                        </span>
                                        <span className="mt-1 inline-flex items-center gap-1 text-xs text-primary">
                                            <Send className="size-3" />
                                            {t('insights.engaged.messages', {
                                                count: c.inbound_30d,
                                            })}
                                        </span>
                                    </Link>
                                </li>
                            ))}
                        </ul>
                    )}
                </CardContent>
            </Card>
        </div>
    );
}

function RateCell({ part, whole }: { part: number; whole: number }) {
    const r = rate(part, whole);
    return (
        <span className="whitespace-nowrap tabular-nums">
            <span className="font-medium">{pct(r)}</span>
            <span className="ml-1 text-xs text-muted-foreground">
                {n(part)}
            </span>
        </span>
    );
}
