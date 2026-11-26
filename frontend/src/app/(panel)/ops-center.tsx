'use client';

import Link from 'next/link';
import { useCallback, useEffect, useRef, useState, useTransition } from 'react';
import {
    AlertTriangle,
    Bot,
    CalendarClock,
    CheckCircle2,
    Clock,
    Hand,
    Inbox,
    Megaphone,
    MessageSquareReply,
    PauseCircle,
    ServerCrash,
    UserRoundX,
    Workflow,
} from 'lucide-react';
import { Area, AreaChart, XAxis } from 'recharts';
import {
    ClientDataTable,
    type ClientColumn,
} from '@/components/client-data-table';
import { useFormatDateTime } from '@/components/display-preferences';
import { notify } from '@/components/notify';
import {
    RESYNCED,
    useRealtimeChannel,
    useRealtimeEvents,
    useRealtimeState,
    type RealtimeEvent,
} from '@/components/realtime';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    ChartContainer,
    ChartTooltip,
    ChartTooltipContent,
    type ChartConfig,
} from '@/components/ui/chart';
import { useT } from '@/i18n/client';
import type { MessageKey, Translate } from '@/i18n/translate';
import { NUMBER_LOCALE } from '@/lib/format';
import type { Schemas } from '@/lib/types';
import { cn } from '@/lib/utils';
import { controlCampaignAction } from './campaigns/actions';
import { ResumeDialog } from './campaigns/[id]/engine';
import {
    CampaignProgress,
    CampaignStatusBadge,
    formatDuration,
    formatRate,
    isPaused,
    pauseReasonLabel,
    priorityLabel,
} from './campaigns/campaign-ui';
import { assignAction, takeOverAction } from './inbox/actions';
import {
    loadOpsAction,
    type OpsOverview,
    type OpsSection,
} from './ops-actions';

type OpsCampaign = Schemas['OpsCampaign'];
type Escalation = Schemas['OpsEscalation'];
type Waiting = Schemas['OpsWaitingConversation'];
type OpsMessage = Schemas['OpsMessage'];
type Run = Schemas['OpsAutomationRun'];

export type OpsPerms = {
    pause: boolean;
    start: boolean;
    reply: boolean;
    assign: boolean;
    automations: boolean;
    channels: boolean;
    aiUsage: boolean;
};

const STATE_LABEL = {
    connected: 'overview.live.connected',
    connecting: 'overview.live.connecting',
    disconnected: 'overview.live.disconnected',
} as const;

const ALL: OpsSection[] = [
    'campaigns',
    'conversations',
    'activity',
    'automations',
    'alerts',
];

/** A translated label, or the raw value when the key is missing. */
function maybe(t: Translate, key: string, raw: string): string {
    const text = t(key as MessageKey);
    return text === key ? raw : text;
}

const fmt = (n: number) => n.toLocaleString(NUMBER_LOCALE);

function money(micros: number, currency: string): string {
    const v = micros / 1_000_000;
    try {
        return currency
            ? v.toLocaleString(NUMBER_LOCALE, {
                  style: 'currency',
                  currency,
                  maximumFractionDigits: 2,
              })
            : fmt(Math.round(v * 100) / 100);
    } catch {
        return `${fmt(Math.round(v * 100) / 100)} ${currency}`;
    }
}

function age(iso: string, now: number): string {
    return formatDuration((now - new Date(iso).getTime()) / 1000);
}

/** The current time, ticking every 30 s for waiting times. */
function useNow(): number {
    const [now, setNow] = useState(() => Date.now());
    useEffect(() => {
        const id = setInterval(() => setNow(Date.now()), 30_000);
        return () => clearInterval(id);
    }, []);
    return now;
}

/**
 * Applies a campaign event (campaign.updated / campaign.progress) to the
 * list; returns whether the section must be read again (a new campaign or
 * a change of state, which moves the counts by state).
 */
function applyCampaign(
    list: OpsCampaign[],
    e: RealtimeEvent,
): { list: OpsCampaign[]; refetch: boolean } {
    const id = typeof e.campaign_id === 'string' ? e.campaign_id : '';
    const i = list.findIndex((c) => c.id === id);
    if (i < 0) return { list, refetch: true };
    const prev = list[i]!;
    const status = (e.status as OpsCampaign['status']) ?? prev.status;
    const next: OpsCampaign = {
        ...prev,
        status,
        counts: (e.counts as Record<string, number>) ?? prev.counts,
        send_rate: (e.rate as number | undefined) ?? prev.send_rate,
        eta_seconds:
            status === 'running'
                ? ((e.eta_seconds as number | undefined) ?? prev.eta_seconds)
                : null,
        paused_reason: (e.paused_reason as string | undefined) ?? null,
        resume_at: (e.resume_at as string | undefined) ?? null,
    };
    const counts = next.counts;
    next.cost_micros = counts.a_cost_micros ?? prev.cost_micros;
    next.spam_errors =
        (counts.a_err_131048 ?? 0) +
        (counts.a_err_131049 ?? 0) +
        (counts.a_err_131050 ?? 0);
    const out = list.slice();
    out[i] = next;
    return { list: out, refetch: status !== prev.status };
}

/**
 * The operations center. It starts from the server's overview and stays
 * current without polling: campaign events update rows in place, inbox
 * events and the ops.updated hints on system:ops re-read only the section
 * that changed (grouped within a second), and a re-sync re-reads all.
 */
export function OpsCenter({
    initial,
    userId,
    timezone,
    perms,
}: {
    initial: OpsOverview;
    userId: string;
    timezone: string;
    perms: OpsPerms;
}) {
    const t = useT();
    const state = useRealtimeState();
    const [data, setData] = useState<OpsOverview>(initial);
    const [failed, setFailed] = useState<string | null>(null);
    useRealtimeChannel('system:ops');

    // Sections waiting to be read again, and a timer grouping bursts.
    const pending = useRef(new Set<OpsSection>());
    const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
    const lastActivity = useRef(0);
    const flush = useCallback(async () => {
        const sections = [...pending.current];
        pending.current.clear();
        if (!sections.length) return;
        const r = await loadOpsAction(sections, timezone);
        if (!r.ok) {
            setFailed(r.message);
            return;
        }
        setFailed(null);
        setData((d) => {
            const next = { ...d, generated_at: r.data.generated_at };
            for (const s of sections) {
                (next as Record<string, unknown>)[s] = r.data[s];
            }
            return next;
        });
    }, [timezone]);
    const refetch = useCallback(
        (...sections: OpsSection[]) => {
            for (const s of sections) pending.current.add(s);
            clearTimeout(timer.current);
            timer.current = setTimeout(() => void flush(), 800);
        },
        [flush],
    );
    useEffect(() => () => clearTimeout(timer.current), []);

    useRealtimeEvents((channel, e) => {
        if (e.type === RESYNCED) {
            refetch(...ALL.filter((s) => data[s] !== undefined));
            return;
        }
        if (e.type === 'ops.updated') {
            const s = e.section as OpsSection;
            if (ALL.includes(s) && data[s] !== undefined) refetch(s);
            return;
        }
        if (
            (e.type === 'campaign.updated' || e.type === 'campaign.progress') &&
            channel === 'campaign:all' &&
            data.campaigns
        ) {
            const res = applyCampaign(data.campaigns.active, e);
            if (res.list !== data.campaigns.active) {
                setData((d) =>
                    d.campaigns
                        ? {
                              ...d,
                              campaigns: { ...d.campaigns, active: res.list },
                          }
                        : d,
                );
            }
            if (res.refetch) refetch('campaigns');
            return;
        }
        const inbox =
            channel.startsWith('inbox:') || channel.startsWith('personal:');
        if (
            inbox &&
            data.conversations &&
            (e.type === 'conversation.updated' ||
                e.type === 'conversation.handoff' ||
                e.type === 'message.created' ||
                e.type === 'notification.created')
        ) {
            const sections: OpsSection[] = ['conversations'];
            // The hourly counts move slowly: at most once a minute.
            if (
                data.activity &&
                e.type === 'message.created' &&
                Date.now() - lastActivity.current > 60_000
            ) {
                lastActivity.current = Date.now();
                sections.push('activity');
            }
            refetch(...sections);
        }
    });

    const empty =
        !data.campaigns &&
        !data.conversations &&
        !data.automations &&
        !data.alerts;
    return (
        <div className="space-y-8" data-testid="ops-center">
            <div className="flex flex-wrap items-center justify-between gap-2">
                <Kpis data={data} />
                <Badge
                    variant={
                        state === 'disconnected' ? 'destructive' : 'secondary'
                    }
                    aria-live="polite"
                    data-testid="realtime-state"
                >
                    {t(STATE_LABEL[state])}
                </Badge>
            </div>
            {failed && (
                <Alert variant="destructive">
                    <AlertDescription>
                        {t('overview.ops.loadFailed', { message: failed })}
                    </AlertDescription>
                </Alert>
            )}
            {empty && (
                <p className="text-sm text-muted-foreground">
                    {t('overview.ops.nothingToShow')}
                </p>
            )}
            {data.alerts && <Alerts alerts={data.alerts} perms={perms} />}
            {data.campaigns && (
                <CampaignsSection
                    section={data.campaigns}
                    perms={perms}
                    timezone={timezone}
                    onChanged={() => refetch('campaigns')}
                />
            )}
            {data.activity && data.activity.hours.length > 0 && (
                <Sparkline hours={data.activity.hours} />
            )}
            {data.conversations && (
                <ConversationsSection
                    section={data.conversations}
                    perms={perms}
                    userId={userId}
                    onChanged={() => refetch('conversations')}
                />
            )}
            {data.automations && (
                <AutomationsSection section={data.automations} />
            )}
        </div>
    );
}

function Kpi({
    icon: Icon,
    label,
    value,
    tone,
    testId,
}: {
    icon: React.ComponentType<{ className?: string }>;
    label: string;
    value: number;
    tone?: 'warn' | 'bad';
    testId: string;
}) {
    return (
        <div
            className="flex min-w-36 items-center gap-3 rounded-lg border bg-card px-3 py-2"
            data-testid={testId}
        >
            <Icon
                className={cn(
                    'size-4 shrink-0 text-muted-foreground',
                    value > 0 && tone === 'warn' && 'text-amber-500',
                    value > 0 && tone === 'bad' && 'text-destructive',
                )}
            />
            <div className="min-w-0">
                <div className="text-lg leading-tight font-semibold tabular-nums">
                    {fmt(value)}
                </div>
                <div className="truncate text-xs text-muted-foreground">
                    {label}
                </div>
            </div>
        </div>
    );
}

function Kpis({ data }: { data: OpsOverview }) {
    const t = useT();
    const c = data.campaigns?.counts;
    const v = data.conversations;
    return (
        <div className="flex flex-wrap gap-2" aria-label={t('overview.title')}>
            {c && (
                <>
                    <Kpi
                        icon={Megaphone}
                        label={t('overview.ops.kpi.running')}
                        value={c.running + c.preparing}
                        testId="kpi-running"
                    />
                    <Kpi
                        icon={PauseCircle}
                        label={t('overview.ops.kpi.paused')}
                        value={c.paused}
                        tone="warn"
                        testId="kpi-paused"
                    />
                    <Kpi
                        icon={CalendarClock}
                        label={t('overview.ops.kpi.scheduled')}
                        value={c.scheduled}
                        testId="kpi-scheduled"
                    />
                    <Kpi
                        icon={CheckCircle2}
                        label={t('overview.ops.kpi.completedToday')}
                        value={c.completed_today}
                        testId="kpi-completed"
                    />
                </>
            )}
            {v && (
                <>
                    <Kpi
                        icon={Hand}
                        label={t('overview.ops.kpi.awaitingHuman')}
                        value={v.awaiting_human_total}
                        tone="bad"
                        testId="kpi-awaiting-human"
                    />
                    <Kpi
                        icon={UserRoundX}
                        label={t('overview.ops.kpi.unassigned')}
                        value={v.unassigned_total}
                        tone="warn"
                        testId="kpi-unassigned"
                    />
                    <Kpi
                        icon={MessageSquareReply}
                        label={t('overview.ops.kpi.waitingReply')}
                        value={v.waiting_reply_total}
                        testId="kpi-waiting-reply"
                    />
                    <Kpi
                        icon={Bot}
                        label={t('overview.ops.kpi.aiActive')}
                        value={v.ai_active}
                        testId="kpi-ai"
                    />
                </>
            )}
            {data.automations && (
                <Kpi
                    icon={Workflow}
                    label={t('overview.ops.kpi.failedRuns')}
                    value={data.automations.failed_total}
                    tone="bad"
                    testId="kpi-failed-runs"
                />
            )}
            {data.alerts && (
                <Kpi
                    icon={ServerCrash}
                    label={t('overview.ops.kpi.failedJobs')}
                    value={data.alerts.failed_jobs}
                    tone="bad"
                    testId="kpi-failed-jobs"
                />
            )}
        </div>
    );
}

function SectionTitle({
    id,
    children,
    action,
}: {
    id: string;
    children: React.ReactNode;
    action?: React.ReactNode;
}) {
    return (
        <div className="mb-3 flex items-center justify-between gap-2">
            <h2 id={id} className="font-medium">
                {children}
            </h2>
            {action}
        </div>
    );
}

function Alerts({
    alerts: a,
    perms,
}: {
    alerts: NonNullable<OpsOverview['alerts']>;
    perms: OpsPerms;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const items: {
        key: string;
        text: string;
        href?: string;
        action?: string;
    }[] = [];
    if (a.failed_jobs > 0) {
        items.push({
            key: 'jobs',
            text: t('overview.ops.alerts.failedJobs', { count: a.failed_jobs }),
            href: '/system/failed-jobs',
            action: t('overview.ops.alerts.failedJobsAction'),
        });
    }
    for (const x of a.accounts) {
        const detail =
            x.status !== 'active'
                ? `${t(`overview.ops.alerts.accountStatus.${x.status}`)}${x.last_error ? ` (${x.last_error})` : ''}`
                : x.token_warning ||
                  (x.token_expires_at
                      ? t('overview.ops.alerts.tokenExpires', {
                            date: formatDateTime(x.token_expires_at),
                        })
                      : '');
        items.push({
            key: `acc-${x.account_id}`,
            text: t('overview.ops.alerts.account', { name: x.name, detail }),
            href: perms.channels ? '/channels' : undefined,
            action: t('overview.ops.alerts.accountAction'),
        });
    }
    for (const x of a.numbers) {
        items.push({
            key: `pn-${x.phone_number_id}`,
            text: t('overview.ops.alerts.quality', {
                number: x.display_phone_number || x.verified_name,
                quality: t(`overview.ops.alerts.qualities.${x.quality_rating}`),
            }),
            href: perms.channels ? '/channels' : undefined,
            action: t('overview.ops.alerts.accountAction'),
        });
    }
    for (const x of a.events) {
        items.push({
            key: `ev-${x.id}`,
            text: `${t('overview.ops.alerts.event', {
                kind: x.kind.replaceAll('_', ' ').replace('.', ': '),
                account: x.display_phone_number || x.account_name,
            })} · ${formatDateTime(x.occurred_at)}`,
        });
    }
    if (a.ai_budget?.exceeded) {
        items.push({
            key: 'budget',
            text: t('overview.ops.alerts.budget', {
                spent: a.ai_budget.spent,
                limit: a.ai_budget.limit ?? '',
            }),
            href: perms.aiUsage ? '/settings/ai/usage' : undefined,
            action: t('overview.ops.alerts.budgetAction'),
        });
    }
    if (a.license.status !== 'active' && a.license.status !== 'trial') {
        items.push({
            key: 'license',
            text: t('overview.ops.alerts.license', {
                status: t(`license.status.${a.license.status}`),
            }),
            href: '/settings/license',
            action: t('overview.ops.alerts.licenseAction'),
        });
    }
    if (!items.length) return null;
    return (
        <section aria-labelledby="ops-alerts">
            <SectionTitle id="ops-alerts">
                {t('overview.ops.alerts.title')}
            </SectionTitle>
            <Alert variant="destructive" data-testid="ops-alerts">
                <AlertTriangle />
                <AlertTitle>
                    {t('overview.ops.alerts.title')} ({items.length})
                </AlertTitle>
                <AlertDescription>
                    <ul className="w-full space-y-1">
                        {items.map((i) => (
                            <li
                                key={i.key}
                                className="flex flex-wrap items-center justify-between gap-2"
                                data-testid="ops-alert"
                            >
                                <span>{i.text}</span>
                                {i.href && (
                                    <Link
                                        href={i.href}
                                        className="text-sm font-medium underline underline-offset-4"
                                    >
                                        {i.action}
                                    </Link>
                                )}
                            </li>
                        ))}
                    </ul>
                </AlertDescription>
            </Alert>
        </section>
    );
}

function PauseButton({ id, onDone }: { id: string; onDone: () => void }) {
    const t = useT();
    const [pending, start] = useTransition();
    return (
        <Button
            size="sm"
            variant="outline"
            disabled={pending}
            data-testid="ops-pause"
            onClick={() =>
                start(async () => {
                    if (notify(await controlCampaignAction(id, 'pause'))) {
                        onDone();
                    }
                })
            }
        >
            <PauseCircle /> {t('overview.ops.campaigns.pause')}
        </Button>
    );
}

function CampaignsSection({
    section,
    perms,
    timezone,
    onChanged,
}: {
    section: NonNullable<OpsOverview['campaigns']>;
    perms: OpsPerms;
    timezone: string;
    onChanged: () => void;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const columns: ClientColumn<OpsCampaign>[] = [
        {
            id: 'name',
            header: t('overview.ops.campaigns.name'),
            sortValue: (c) => c.name,
            cell: (c) => (
                <div className="min-w-0">
                    <Link
                        href={`/campaigns/${c.id}`}
                        className="font-medium hover:underline"
                    >
                        {c.name}
                    </Link>
                    <div className="text-xs text-muted-foreground">
                        {c.display_phone_number} ·{' '}
                        {priorityLabel(t, c.priority)}
                    </div>
                </div>
            ),
        },
        {
            id: 'status',
            header: t('overview.ops.campaigns.status'),
            sortValue: (c) => c.status,
            cell: (c) => (
                <div className="space-y-1">
                    <CampaignStatusBadge status={c.status} />
                    {c.paused_reason && isPaused(c.status) && (
                        <div className="text-xs text-muted-foreground">
                            {pauseReasonLabel(t, c.paused_reason)}
                        </div>
                    )}
                    {c.status === 'scheduled' && c.scheduled_at && (
                        <div className="text-xs text-muted-foreground">
                            {t('overview.ops.campaigns.scheduledFor', {
                                at: formatDateTime(c.scheduled_at),
                            })}
                        </div>
                    )}
                </div>
            ),
        },
        {
            id: 'progress',
            header: t('overview.ops.campaigns.progress'),
            className: 'min-w-48',
            cell: (c) => (
                <CampaignProgress
                    counts={c.counts}
                    total={c.total_recipients}
                    compact
                    label={c.name}
                />
            ),
        },
        {
            id: 'rate',
            header: t('overview.ops.campaigns.rate'),
            sortValue: (c) => c.send_rate ?? 0,
            cell: (c) => (
                <span className="tabular-nums" data-testid="ops-rate">
                    {c.send_rate
                        ? t('overview.ops.campaigns.perSecond', {
                              rate: formatRate(c.send_rate),
                          })
                        : '—'}
                </span>
            ),
        },
        {
            id: 'eta',
            header: t('overview.ops.campaigns.eta'),
            sortValue: (c) => c.eta_seconds ?? Infinity,
            cell: (c) => (
                <span className="tabular-nums">
                    {c.eta_seconds != null
                        ? formatDuration(c.eta_seconds)
                        : '—'}
                </span>
            ),
        },
        {
            id: 'cost',
            header: t('overview.ops.campaigns.cost'),
            sortValue: (c) => c.cost_micros,
            cell: (c) => (
                <span className="tabular-nums">
                    {money(c.cost_micros, c.currency)}
                </span>
            ),
        },
        {
            id: 'signals',
            header: t('overview.ops.campaigns.warnings'),
            cell: (c) =>
                c.spam_errors > 0 || c.paused_reason === 'spam_protection' ? (
                    <Badge variant="destructive" data-testid="ops-spam">
                        <AlertTriangle />
                        {t('overview.ops.campaigns.spam', {
                            count: c.spam_errors,
                        })}
                    </Badge>
                ) : c.last_error ? (
                    <span className="text-xs text-destructive">
                        {c.last_error}
                    </span>
                ) : (
                    '—'
                ),
        },
    ];
    return (
        <section aria-labelledby="ops-campaigns" className="space-y-4">
            <SectionTitle
                id="ops-campaigns"
                action={
                    <Button asChild size="sm" variant="ghost">
                        <Link href="/campaigns">
                            {t('overview.ops.campaigns.all')}
                        </Link>
                    </Button>
                }
            >
                {t('overview.ops.campaigns.title')}
            </SectionTitle>
            <ClientDataTable
                rows={section.active}
                columns={columns}
                rowId={(c) => c.id}
                storageKey="ops-campaigns"
                pageSize={10}
                rowTestId="ops-campaign-row"
                rowActions={(c) => (
                    <div className="flex justify-end gap-2">
                        {perms.pause &&
                            (c.status === 'running' ||
                                c.status === 'scheduled') && (
                                <PauseButton id={c.id} onDone={onChanged} />
                            )}
                        {perms.start && isPaused(c.status) && (
                            <ResumeDialog
                                campaign={c}
                                timezone={timezone}
                                onResumed={onChanged}
                            />
                        )}
                    </div>
                )}
                empty={
                    <p className="py-6 text-center text-sm text-muted-foreground">
                        {t('overview.ops.campaigns.empty')}
                    </p>
                }
            />
            {section.queue.length > 0 && (
                <div>
                    <h3 className="mb-2 text-sm font-medium text-muted-foreground">
                        {t('overview.ops.campaigns.queue')}
                    </h3>
                    <ul className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
                        {section.queue.map((n) => {
                            const pn = section.active.find(
                                (c) => c.phone_number_id === n.phone_number_id,
                            )?.display_phone_number;
                            const share =
                                n.campaign_capacity_mps > 0
                                    ? n.campaigns.reduce(
                                          (s, c) => s + c.share_mps,
                                          0,
                                      ) / n.campaign_capacity_mps
                                    : 0;
                            return (
                                <li
                                    key={n.phone_number_id}
                                    className="rounded-lg border bg-card p-3 text-sm"
                                    data-testid="ops-queue-number"
                                >
                                    <div className="font-medium">
                                        {pn ?? n.phone_number_id.slice(0, 8)}
                                    </div>
                                    <div className="text-xs text-muted-foreground">
                                        {t('overview.ops.campaigns.queueLine', {
                                            rate: fmt(n.rate_mps),
                                            ceiling: fmt(n.ceiling_mps),
                                            capacity: formatRate(
                                                n.campaign_capacity_mps,
                                            ),
                                        })}
                                    </div>
                                    <div
                                        className="mt-2 h-1.5 overflow-hidden rounded bg-muted"
                                        aria-hidden
                                    >
                                        <div
                                            className="h-full bg-primary"
                                            style={{
                                                width: `${Math.min(100, Math.round(share * 100))}%`,
                                            }}
                                        />
                                    </div>
                                    <div className="mt-2 text-xs text-muted-foreground">
                                        {n.limit.limit > 0
                                            ? t(
                                                  'overview.ops.campaigns.limit',
                                                  {
                                                      used: fmt(n.limit.used),
                                                      limit: fmt(n.limit.limit),
                                                  },
                                              )
                                            : t(
                                                  'overview.ops.campaigns.noLimit',
                                              )}
                                    </div>
                                </li>
                            );
                        })}
                    </ul>
                </div>
            )}
        </section>
    );
}

function Sparkline({
    hours,
}: {
    hours: NonNullable<OpsOverview['activity']>['hours'];
}) {
    const t = useT();
    const config: ChartConfig = {
        outbound: {
            label: t('overview.ops.activity.outbound'),
            color: 'var(--chart-1)',
        },
        inbound: {
            label: t('overview.ops.activity.inbound'),
            color: 'var(--chart-2)',
        },
    };
    const data = hours.map((h) => ({
        label: new Date(h.hour).toLocaleTimeString(NUMBER_LOCALE, {
            hour: '2-digit',
            minute: '2-digit',
        }),
        outbound: h.outbound,
        inbound: h.inbound,
    }));
    return (
        <section aria-labelledby="ops-activity">
            <SectionTitle id="ops-activity">
                {t('overview.ops.activity.title')}
            </SectionTitle>
            <ChartContainer
                config={config}
                className="aspect-auto h-28 w-full"
                data-testid="ops-sparkline"
            >
                <AreaChart data={data} margin={{ left: 0, right: 0, top: 4 }}>
                    <XAxis dataKey="label" hide />
                    <ChartTooltip content={<ChartTooltipContent />} />
                    <Area
                        dataKey="outbound"
                        type="monotone"
                        stroke="var(--color-outbound)"
                        fill="var(--color-outbound)"
                        fillOpacity={0.15}
                    />
                    <Area
                        dataKey="inbound"
                        type="monotone"
                        stroke="var(--color-inbound)"
                        fill="var(--color-inbound)"
                        fillOpacity={0.15}
                    />
                </AreaChart>
            </ChartContainer>
        </section>
    );
}

function Customer({ name, phone }: { name: string; phone: string }) {
    return (
        <div className="min-w-0">
            <div className="truncate font-medium">{name || phone}</div>
            {name && (
                <div className="text-xs text-muted-foreground tabular-nums">
                    {phone}
                </div>
            )}
        </div>
    );
}

function OpenLink({ id }: { id: string }) {
    const t = useT();
    return (
        <Button asChild size="sm" variant="ghost">
            <Link href={`/inbox?c=${id}`}>
                {t('overview.ops.conversations.open')}
            </Link>
        </Button>
    );
}

function ActionButton({
    label,
    run,
    onDone,
    testId,
}: {
    label: string;
    run: () => Promise<Parameters<typeof notify>[0]>;
    onDone: () => void;
    testId: string;
}) {
    const [pending, start] = useTransition();
    return (
        <Button
            size="sm"
            disabled={pending}
            data-testid={testId}
            onClick={() =>
                start(async () => {
                    if (notify(await run())) onDone();
                })
            }
        >
            {label}
        </Button>
    );
}

function ConversationsSection({
    section: s,
    perms,
    userId,
    onChanged,
}: {
    section: NonNullable<OpsOverview['conversations']>;
    perms: OpsPerms;
    userId: string;
    onChanged: () => void;
}) {
    const t = useT();
    const now = useNow();
    const customer = t('overview.ops.conversations.customer');
    const escalations: ClientColumn<Escalation>[] = [
        {
            id: 'customer',
            header: customer,
            sortValue: (e) => e.contact_name || e.phone_e164,
            cell: (e) => (
                <Customer name={e.contact_name} phone={e.phone_e164} />
            ),
        },
        {
            id: 'reason',
            header: t('overview.ops.conversations.reason'),
            cell: (e) => (
                <div className="min-w-0">
                    <div>
                        {maybe(t, `inbox.ai.reasons.${e.reason}`, e.reason)}
                    </div>
                    {e.reason_detail && (
                        <div className="truncate text-xs text-muted-foreground">
                            {e.reason_detail}
                        </div>
                    )}
                </div>
            ),
        },
        {
            id: 'age',
            header: t('overview.ops.conversations.waiting'),
            sortValue: (e) => e.escalated_at,
            cell: (e) => (
                <span className="tabular-nums">{age(e.escalated_at, now)}</span>
            ),
        },
    ];
    const waitingCols = (assignee: boolean): ClientColumn<Waiting>[] => [
        {
            id: 'customer',
            header: customer,
            sortValue: (w) => w.contact_name || w.phone_e164,
            cell: (w) => (
                <Customer name={w.contact_name} phone={w.phone_e164} />
            ),
        },
        {
            id: 'age',
            header: t('overview.ops.conversations.waiting'),
            sortValue: (w) => w.waiting_since,
            cell: (w) => (
                <span className="tabular-nums">
                    {age(w.waiting_since, now)}
                </span>
            ),
        },
        ...(assignee
            ? [
                  {
                      id: 'assignee',
                      header: t('overview.ops.conversations.assignee'),
                      cell: (w: Waiting) =>
                          w.owner_type === 'ai' ? (
                              <Badge variant="secondary">
                                  <Bot /> AI
                              </Badge>
                          ) : (
                              (w.assignee_name ??
                              t('overview.ops.conversations.nobody'))
                          ),
                  },
              ]
            : []),
    ];
    return (
        <section aria-labelledby="ops-conversations" className="space-y-6">
            <div className="grid gap-6 xl:grid-cols-2">
                <div data-testid="ops-awaiting-human">
                    <SectionTitle
                        id="ops-conversations"
                        action={
                            <Button asChild size="sm" variant="ghost">
                                <Link href="/inbox?ai=waiting_human">
                                    <Inbox />{' '}
                                    {t('overview.ops.conversations.all')}
                                </Link>
                            </Button>
                        }
                    >
                        {t('overview.ops.conversations.awaitingHuman')}
                    </SectionTitle>
                    <ClientDataTable
                        rows={s.awaiting_human}
                        columns={escalations}
                        rowId={(e) => e.conversation_id}
                        storageKey="ops-escalations"
                        pageSize={10}
                        rowTestId="ops-escalation-row"
                        rowActions={(e) => (
                            <div className="flex justify-end gap-1">
                                {perms.reply && (
                                    <ActionButton
                                        label={t(
                                            'overview.ops.conversations.takeOver',
                                        )}
                                        testId="ops-take-over"
                                        run={() =>
                                            takeOverAction(e.conversation_id)
                                        }
                                        onDone={onChanged}
                                    />
                                )}
                                <OpenLink id={e.conversation_id} />
                            </div>
                        )}
                        empty={
                            <p className="py-6 text-center text-sm text-muted-foreground">
                                {t('overview.ops.conversations.awaitingEmpty')}
                            </p>
                        }
                    />
                </div>
                <div data-testid="ops-unassigned">
                    <SectionTitle
                        id="ops-unassigned-title"
                        action={
                            <Button asChild size="sm" variant="ghost">
                                <Link href="/inbox?assignee=unassigned">
                                    <Inbox />{' '}
                                    {t('overview.ops.conversations.all')}
                                </Link>
                            </Button>
                        }
                    >
                        {t('overview.ops.conversations.unassigned')}
                    </SectionTitle>
                    <ClientDataTable
                        rows={s.unassigned}
                        columns={waitingCols(false)}
                        rowId={(w) => w.conversation_id}
                        storageKey="ops-unassigned"
                        pageSize={10}
                        rowTestId="ops-unassigned-row"
                        rowActions={(w) => (
                            <div className="flex justify-end gap-1">
                                {perms.assign && (
                                    <ActionButton
                                        label={t(
                                            'overview.ops.conversations.assignMe',
                                        )}
                                        testId="ops-assign-me"
                                        run={() =>
                                            assignAction(
                                                w.conversation_id,
                                                userId,
                                            )
                                        }
                                        onDone={onChanged}
                                    />
                                )}
                                <OpenLink id={w.conversation_id} />
                            </div>
                        )}
                        empty={
                            <p className="py-6 text-center text-sm text-muted-foreground">
                                {t(
                                    'overview.ops.conversations.unassignedEmpty',
                                )}
                            </p>
                        }
                    />
                </div>
            </div>
            <div>
                <SectionTitle id="ops-waiting-reply">
                    {t('overview.ops.conversations.waitingReply')}
                </SectionTitle>
                <ClientDataTable
                    rows={s.waiting_reply}
                    columns={waitingCols(true)}
                    rowId={(w) => w.conversation_id}
                    storageKey="ops-waiting-reply"
                    pageSize={10}
                    rowTestId="ops-waiting-row"
                    rowActions={(w) => <OpenLink id={w.conversation_id} />}
                    empty={
                        <p className="py-6 text-center text-sm text-muted-foreground">
                            {t('overview.ops.conversations.waitingEmpty')}
                        </p>
                    }
                />
            </div>
            <div className="grid gap-6 xl:grid-cols-2">
                <Recent
                    title={t('overview.ops.conversations.recentInbound')}
                    rows={s.recent_inbound}
                    testId="ops-recent-inbound"
                />
                <Recent
                    title={t('overview.ops.conversations.recentOutbound')}
                    rows={s.recent_outbound}
                    testId="ops-recent-outbound"
                />
            </div>
        </section>
    );
}

function Recent({
    title,
    rows,
    testId,
}: {
    title: string;
    rows: OpsMessage[];
    testId: string;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    return (
        <div data-testid={testId}>
            <h3 className="mb-2 text-sm font-medium text-muted-foreground">
                {title}
            </h3>
            {rows.length === 0 ? (
                <p className="text-sm text-muted-foreground">
                    {t('overview.ops.conversations.recentEmpty')}
                </p>
            ) : (
                <ul className="divide-y rounded-lg border bg-card text-sm">
                    {rows.map((m) => {
                        const body = (
                            <>
                                <span className="w-32 shrink-0 truncate font-medium">
                                    {m.contact_name || m.phone_e164}
                                </span>
                                <span className="min-w-0 flex-1 truncate text-muted-foreground">
                                    {m.preview ||
                                        t('overview.ops.conversations.media', {
                                            type: m.type,
                                        })}
                                </span>
                                <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
                                    {m.error_code ? (
                                        <span className="text-destructive">
                                            {m.error_code}
                                        </span>
                                    ) : (
                                        maybe(
                                            t,
                                            `overview.live.messageStatus.${m.status}`,
                                            m.status,
                                        )
                                    )}{' '}
                                    · {formatDateTime(m.created_at)}
                                </span>
                            </>
                        );
                        return (
                            <li key={m.id}>
                                {m.conversation_id ? (
                                    <Link
                                        href={`/inbox?c=${m.conversation_id}`}
                                        className="flex items-center gap-3 px-3 py-2 hover:bg-muted/50"
                                    >
                                        {body}
                                    </Link>
                                ) : (
                                    <div className="flex items-center gap-3 px-3 py-2">
                                        {body}
                                    </div>
                                )}
                            </li>
                        );
                    })}
                </ul>
            )}
        </div>
    );
}

function AutomationsSection({
    section: s,
}: {
    section: NonNullable<OpsOverview['automations']>;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const name: ClientColumn<Run> = {
        id: 'automation',
        header: t('overview.ops.automations.automation'),
        sortValue: (r) => r.automation_name,
        cell: (r) => (
            <Link
                href={`/automations/${r.automation_id}/runs/${r.id}`}
                className="font-medium hover:underline"
            >
                {r.automation_name}
            </Link>
        ),
    };
    const running: ClientColumn<Run>[] = [
        name,
        {
            id: 'status',
            header: t('overview.ops.automations.status'),
            cell: (r) => (
                <Badge variant="secondary">
                    <Clock />
                    {t(`overview.ops.automations.statuses.${r.status}`)}
                </Badge>
            ),
        },
        {
            id: 'at',
            header: t('overview.ops.automations.started'),
            sortValue: (r) => r.at,
            cell: (r) => formatDateTime(r.at),
        },
    ];
    const failed: ClientColumn<Run>[] = [
        name,
        {
            id: 'error',
            header: t('overview.ops.automations.error'),
            cell: (r) => (
                <div className="min-w-0" data-testid="ops-run-error">
                    {r.error_code && (
                        <Badge variant="destructive">
                            {maybe(
                                t,
                                `overview.ops.automations.codes.${r.error_code}`,
                                r.error_code,
                            )}
                        </Badge>
                    )}
                    {r.error && (
                        <div className="truncate text-xs text-muted-foreground">
                            {r.error}
                        </div>
                    )}
                    {!!r.attempt && (
                        <div className="text-xs text-muted-foreground">
                            {t('overview.ops.automations.attempt', {
                                count: r.attempt,
                            })}
                        </div>
                    )}
                </div>
            ),
        },
        {
            id: 'at',
            header: t('overview.ops.automations.failedAt'),
            sortValue: (r) => r.at,
            cell: (r) => formatDateTime(r.at),
        },
    ];
    const open = (r: Run) =>
        r.conversation_id ? <OpenLink id={r.conversation_id} /> : null;
    return (
        <section
            aria-labelledby="ops-automations"
            className="grid gap-6 xl:grid-cols-2"
        >
            <div data-testid="ops-failed-runs">
                <SectionTitle
                    id="ops-automations"
                    action={
                        <Button asChild size="sm" variant="ghost">
                            <Link href="/automations">
                                {t('overview.ops.automations.runs')}
                            </Link>
                        </Button>
                    }
                >
                    {t('overview.ops.automations.failed')}
                </SectionTitle>
                <ClientDataTable
                    rows={s.failed}
                    columns={failed}
                    rowId={(r) => r.id}
                    storageKey="ops-failed-runs"
                    pageSize={10}
                    rowTestId="ops-failed-run-row"
                    rowActions={open}
                    empty={
                        <p className="py-6 text-center text-sm text-muted-foreground">
                            {t('overview.ops.automations.failedEmpty')}
                        </p>
                    }
                />
            </div>
            <div data-testid="ops-running-runs">
                <SectionTitle id="ops-running-runs-title">
                    {t('overview.ops.automations.running')}
                </SectionTitle>
                <ClientDataTable
                    rows={s.running}
                    columns={running}
                    rowId={(r) => r.id}
                    storageKey="ops-running-runs"
                    pageSize={10}
                    rowActions={open}
                    empty={
                        <p className="py-6 text-center text-sm text-muted-foreground">
                            {t('overview.ops.automations.runningEmpty')}
                        </p>
                    }
                />
            </div>
        </section>
    );
}
