'use client';

import { useEffect, useRef, useState, useTransition } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import {
    AlertTriangle,
    ArrowLeft,
    HelpCircle,
    Pause,
    Pencil,
    Play,
    Square,
    Trash2,
} from 'lucide-react';
import { newIdempotencyKey, notify } from '@/components/notify';
import { useRealtimeEvents, useRealtimeState } from '@/components/realtime';
import { useReauth } from '@/components/reauth';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import {
    AlertDialog,
    AlertDialogAction,
    AlertDialogCancel,
    AlertDialogContent,
    AlertDialogDescription,
    AlertDialogFooter,
    AlertDialogHeader,
    AlertDialogTitle,
    AlertDialogTrigger,
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Card,
    CardContent,
    CardDescription,
    CardHeader,
    CardTitle,
} from '@/components/ui/card';
import { Spinner } from '@/components/ui/spinner';
import {
    Tooltip,
    TooltipContent,
    TooltipTrigger,
} from '@/components/ui/tooltip';
import { useFormatDateTime } from '@/components/display-preferences';
import { useT } from '@/i18n/client';
import type { Translate } from '@/i18n/translate';
import { formatInZone } from '@/lib/timezone';
import type { ActionResult, Campaign, CampaignEventItem } from '@/lib/types';
import {
    controlCampaignAction,
    deleteCampaignAction,
    startCampaignAction,
} from '../actions';
import {
    audienceLabel,
    CampaignProgress,
    CampaignStatusBadge,
    summarize,
    isPaused,
} from '../campaign-ui';
import type { CampaignPause } from '../actions';
import {
    EngineSummary,
    PauseHistory,
    PauseNotice,
    ResumeDialog,
} from './engine';
import { NUMBER_LOCALE } from '@/lib/format';
import { CampaignAnalysis } from './analysis';
import { RecipientsPanel } from './recipients-panel';

type Perms = { edit: boolean; start: boolean; pause: boolean; del: boolean };

export function CampaignDashboard({
    campaign,
    events,
    sender,
    timezone,
    perms,
    pauses,
}: {
    campaign: Campaign;
    pauses: CampaignPause[];
    events: CampaignEventItem[];
    sender?: string;
    timezone: string;
    perms: Perms;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const router = useRouter();
    const rt = useRealtimeState();
    // Counts and status straight from realtime events, until the next
    // server render brings the full campaign again.
    const [live, setLive] = useState<Partial<
        Pick<
            Campaign,
            | 'status'
            | 'counts'
            | 'send_rate'
            | 'eta_seconds'
            | 'paused_reason'
            | 'resume_at'
        >
    > | null>(null);
    const [prev, setPrev] = useState(campaign);
    if (prev !== campaign) {
        setPrev(campaign);
        setLive(null);
    }
    const c = { ...campaign, ...(live ?? {}) };
    const refreshTimer = useRef<ReturnType<typeof setTimeout> | undefined>(
        undefined,
    );
    useEffect(() => () => clearTimeout(refreshTimer.current), []);

    // campaign.updated (state changes) and campaign.progress (live counters,
    // rate and ETA, at most every two seconds) are published to campaign:all
    // and campaign:<id> with the same payload; the panel already listens on
    // campaign:all.
    useRealtimeEvents((channel, e) => {
        if (
            (e.type !== 'campaign.updated' && e.type !== 'campaign.progress') ||
            e.campaign_id !== campaign.id ||
            (channel !== 'campaign:all' &&
                channel !== `campaign:${campaign.id}`)
        ) {
            return;
        }
        const status =
            typeof e.status === 'string'
                ? (e.status as Campaign['status'])
                : c.status;
        const counts =
            e.counts && typeof e.counts === 'object'
                ? (e.counts as Record<string, number>)
                : c.counts;
        const num = (v: unknown) => (typeof v === 'number' ? v : null);
        const str = (v: unknown) => (typeof v === 'string' ? v : null);
        setLive({
            status,
            counts,
            send_rate: num(e.rate) ?? c.send_rate,
            eta_seconds: num(e.eta_seconds),
            paused_reason: str(e.paused_reason),
            resume_at: str(e.resume_at),
        });
        // A new state means new log entries and timestamps: re-read them.
        if (status !== c.status) {
            clearTimeout(refreshTimer.current);
            refreshTimer.current = setTimeout(() => router.refresh(), 300);
        }
    });

    const s = summarize(c.counts, c.total_recipients);
    const notStarted = c.status === 'draft' || c.status === 'scheduled';

    return (
        <div className="space-y-6">
            <div>
                <Link
                    href="/campaigns"
                    className="inline-flex items-center gap-1 text-sm text-muted-foreground hover:text-foreground"
                >
                    <ArrowLeft className="size-4" /> {t('nav.campaigns')}
                </Link>
            </div>
            <div className="flex flex-wrap items-start justify-between gap-4">
                <div className="space-y-1">
                    <div className="flex flex-wrap items-center gap-2">
                        <h1 className="text-2xl font-semibold tracking-tight">
                            {c.name}
                        </h1>
                        <CampaignStatusBadge status={c.status} />
                        <Badge
                            variant="outline"
                            className="text-xs"
                            data-testid="realtime-badge"
                        >
                            {rt === 'connected'
                                ? t('campaigns.dashboard.live')
                                : rt === 'connecting'
                                  ? t('campaigns.dashboard.connecting')
                                  : t('campaigns.dashboard.offline')}
                        </Badge>
                    </div>
                    <p className="text-sm text-muted-foreground">
                        <span className="font-mono">
                            {c.template_name} ({c.template_language})
                        </span>
                        {' · '}
                        {audienceLabel(t, c.audience)}
                        {sender
                            ? ` · ${t('campaigns.dashboard.from', { sender })}`
                            : ''}
                    </p>
                </div>
                <Controls campaign={c} perms={perms} timezone={timezone} />
            </div>

            <PauseNotice campaign={c} pauses={pauses} />

            {c.last_error && (
                <Alert variant="destructive">
                    <AlertTriangle />
                    <AlertTitle>
                        {isPaused(c.status)
                            ? t('campaigns.dashboard.pausedAutomatically')
                            : t('campaigns.dashboard.lastError')}
                    </AlertTitle>
                    <AlertDescription>{c.last_error}</AlertDescription>
                </Alert>
            )}

            <Card>
                <CardHeader>
                    <CardTitle>{t('campaigns.dashboard.progress')}</CardTitle>
                    <CardDescription>
                        {c.status === 'scheduled' && c.scheduled_at
                            ? t('campaigns.dashboard.startsAt', {
                                  date: formatInZone(c.scheduled_at, timezone),
                              })
                            : notStarted
                              ? t('campaigns.dashboard.collectedOnStart')
                              : c.finished_at
                                ? t('campaigns.dashboard.finishedAt', {
                                      date: formatDateTime(c.finished_at),
                                  })
                                : c.started_at
                                  ? t('campaigns.dashboard.startedAt', {
                                        date: formatDateTime(c.started_at),
                                    })
                                  : t('campaigns.dashboard.collecting')}
                    </CardDescription>
                </CardHeader>
                <CardContent className="space-y-6">
                    <EngineSummary campaign={c} canChange={perms.start} />
                    <CampaignProgress
                        counts={c.counts}
                        total={c.total_recipients}
                        label={t('campaigns.dashboard.campaignProgress')}
                    />
                    <dl
                        className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6"
                        data-testid="campaign-counts"
                    >
                        <Stat id="recipients" value={s.recipients} />
                        <Stat id="skipped" value={s.skipped} />
                        <Stat id="waiting" value={s.waiting} />
                        <Stat id="sending" value={s.inFlight} />
                        <Stat id="sent" value={s.sent} />
                        <Stat id="delivered" value={s.delivered} />
                        <Stat id="read" value={s.read} />
                        <Stat id="failed" value={s.failed} tone="bad" />
                        <Stat
                            id="unknown"
                            value={s.unknown}
                            tone="warn"
                            help={t('campaigns.dashboard.unknownHelp')}
                        />
                        <Stat
                            id="held"
                            value={s.held}
                            help={t('campaigns.dashboard.heldHelp')}
                        />
                        <Stat
                            id="deferred"
                            value={s.deferred}
                            tone="warn"
                            help={t('campaigns.dashboard.deferredHelp')}
                        />
                        <Stat id="cancelled" value={s.cancelled} />
                    </dl>
                    <p className="text-xs text-muted-foreground">
                        {t('campaigns.dashboard.window', {
                            size:
                                c.window_size ||
                                t('campaigns.dashboard.automatic'),
                            date: formatDateTime(c.created_at),
                        })}
                    </p>
                </CardContent>
            </Card>

            {!notStarted && (
                <CampaignAnalysis
                    campaign={c}
                    counts={c.counts}
                    perms={{ edit: perms.edit, start: perms.start }}
                />
            )}

            {!notStarted && (
                <section
                    className="min-w-0 space-y-3"
                    aria-labelledby="campaign-recipients"
                >
                    <div>
                        <h2
                            id="campaign-recipients"
                            className="text-lg font-semibold tracking-tight"
                        >
                            {t('campaigns.dashboard.recipients')}
                        </h2>
                        <p className="text-sm text-muted-foreground">
                            {t('campaigns.dashboard.recipientsHint')}
                        </p>
                    </div>
                    <RecipientsPanel campaign={c} canEdit={perms.edit} />
                </section>
            )}

            <Card>
                <CardHeader>
                    <CardTitle>{t('campaigns.dashboard.eventLog')}</CardTitle>
                </CardHeader>
                <CardContent>
                    <EventLog events={events} />
                </CardContent>
            </Card>
            {!notStarted && <PauseHistory pauses={pauses} />}
        </div>
    );
}

function Stat({
    id,
    value,
    tone,
    help,
}: {
    id:
        | 'recipients'
        | 'skipped'
        | 'waiting'
        | 'sending'
        | 'sent'
        | 'delivered'
        | 'read'
        | 'failed'
        | 'unknown'
        | 'held'
        | 'deferred'
        | 'cancelled';
    value: number;
    tone?: 'bad' | 'warn';
    help?: string;
}) {
    const t = useT();
    const label = t(`campaigns.counts.${id}`);
    return (
        <div className="rounded-lg border p-3" data-testid={`count-${id}`}>
            <dt className="flex items-center gap-1 text-xs text-muted-foreground">
                {label}
                {help && (
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <button
                                type="button"
                                className="rounded-full text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                                aria-label={t(
                                    'campaigns.dashboard.whatDoesMean',
                                    {
                                        label,
                                    },
                                )}
                            >
                                <HelpCircle className="size-3.5" />
                            </button>
                        </TooltipTrigger>
                        <TooltipContent className="max-w-xs">
                            {help}
                        </TooltipContent>
                    </Tooltip>
                )}
            </dt>
            <dd
                className={`text-xl font-semibold tabular-nums ${
                    value > 0 && tone === 'bad'
                        ? 'text-destructive'
                        : value > 0 && tone === 'warn'
                          ? 'text-amber-600 dark:text-amber-400'
                          : ''
                }`}
            >
                {value.toLocaleString(NUMBER_LOCALE)}
            </dd>
        </div>
    );
}

// ── Controls ───────────────────────────────────────────────────────────────

function ConfirmButton({
    label,
    icon,
    title,
    description,
    confirm,
    destructive,
    variant = 'outline',
    run,
}: {
    label: string;
    icon: React.ReactNode;
    title: string;
    description: string;
    confirm: string;
    destructive?: boolean;
    variant?: 'default' | 'outline';
    run: () => Promise<ActionResult<unknown>>;
}) {
    const t = useT();
    const [open, setOpen] = useState(false);
    const [pending, startTransition] = useTransition();
    return (
        <AlertDialog open={open} onOpenChange={setOpen}>
            <AlertDialogTrigger asChild>
                <Button
                    variant={variant}
                    className={destructive ? 'text-destructive' : undefined}
                >
                    {icon} {label}
                </Button>
            </AlertDialogTrigger>
            <AlertDialogContent>
                <AlertDialogHeader>
                    <AlertDialogTitle>{title}</AlertDialogTitle>
                    <AlertDialogDescription>
                        {description}
                    </AlertDialogDescription>
                </AlertDialogHeader>
                <AlertDialogFooter>
                    <AlertDialogCancel disabled={pending}>
                        {t('campaigns.dashboard.keepAsIs')}
                    </AlertDialogCancel>
                    <AlertDialogAction
                        variant={destructive ? 'destructive' : 'default'}
                        disabled={pending}
                        onClick={(e) => {
                            e.preventDefault();
                            startTransition(async () => {
                                if (notify(await run())) setOpen(false);
                            });
                        }}
                    >
                        {pending && <Spinner />} {confirm}
                    </AlertDialogAction>
                </AlertDialogFooter>
            </AlertDialogContent>
        </AlertDialog>
    );
}

function Controls({
    campaign: c,
    perms,
    timezone,
}: {
    campaign: Campaign;
    perms: Perms;
    timezone: string;
}) {
    const t = useT();
    const router = useRouter();
    const { guard } = useReauth();
    const editable = c.status === 'draft' || c.status === 'scheduled';
    const active = c.status === 'running' || c.status === 'preparing';
    const paused = isPaused(c.status);
    const deletable = !['running', 'preparing'].includes(c.status) && !paused;
    const control = (op: 'pause' | 'cancel') => async () => {
        const r = await guard(() => controlCampaignAction(c.id, op));
        if (r.ok) router.refresh();
        return r;
    };
    return (
        <div className="flex flex-wrap gap-2">
            {editable && perms.edit && (
                <Button variant="outline" asChild>
                    <Link href={`/campaigns/${c.id}/edit`}>
                        <Pencil /> {t('common.edit')}
                    </Link>
                </Button>
            )}
            {editable && perms.start && (
                <ConfirmButton
                    variant="default"
                    label={t('campaigns.dashboard.startNow')}
                    icon={<Play />}
                    title={t('campaigns.dashboard.startTitle')}
                    description={t('campaigns.dashboard.startDescription')}
                    confirm={t('campaigns.dashboard.startNow')}
                    run={async () => {
                        const r = await guard(() =>
                            startCampaignAction(
                                c.id,
                                null,
                                newIdempotencyKey(),
                            ),
                        );
                        if (r.ok) router.refresh();
                        return r;
                    }}
                />
            )}
            {(active || c.status === 'paused_limit') && perms.pause && (
                <ConfirmButton
                    label={t('campaigns.dashboard.pause')}
                    icon={<Pause />}
                    title={t('campaigns.dashboard.pauseTitle')}
                    description={t('campaigns.dashboard.pauseDescription')}
                    confirm={t('campaigns.dashboard.pause')}
                    run={control('pause')}
                />
            )}
            {paused && perms.start && (
                <ResumeDialog campaign={c} timezone={timezone} />
            )}
            {(active || paused || c.status === 'scheduled') && perms.pause && (
                <ConfirmButton
                    destructive
                    label={
                        c.status === 'scheduled'
                            ? t('campaigns.dashboard.cancelSchedule')
                            : t('campaigns.dashboard.cancel')
                    }
                    icon={<Square />}
                    title={t('campaigns.dashboard.cancelTitle')}
                    description={t('campaigns.dashboard.cancelDescription')}
                    confirm={t('campaigns.dashboard.cancelConfirm')}
                    run={control('cancel')}
                />
            )}
            {deletable && perms.del && (
                <ConfirmButton
                    destructive
                    label={t('common.delete')}
                    icon={<Trash2 />}
                    title={t('campaigns.dashboard.deleteTitle')}
                    description={t('campaigns.dashboard.deleteDescription')}
                    confirm={t('common.delete')}
                    run={async () => {
                        const r = await guard(() => deleteCampaignAction(c.id));
                        if (r.ok) router.push('/campaigns');
                        return r;
                    }}
                />
            )}
        </div>
    );
}

// ── Event log ──────────────────────────────────────────────────────────────

const EVENT_KINDS = [
    'scheduled',
    'started',
    'paused',
    'auto_paused',
    'resumed',
    'cancelled',
    'completed',
    'failed',
] as const;

function eventLabel(t: Translate, kind: string): string {
    const k = EVENT_KINDS.find((x) => x === kind);
    return k ? t(`campaigns.dashboard.events.${k}`) : kind;
}

function describeDetail(
    t: Translate,
    d: Record<string, unknown>,
    formatDateTime: (iso: string | null | undefined) => string,
): string {
    const parts: string[] = [];
    if (typeof d.reason === 'string') parts.push(d.reason);
    if (typeof d.error === 'string') parts.push(d.error);
    if (typeof d.at === 'string')
        parts.push(
            t('campaigns.dashboard.eventFor', { date: formatDateTime(d.at) }),
        );
    if (d.scheduled === true) parts.push(t('campaigns.dashboard.onSchedule'));
    return parts.join(' · ');
}

function EventLog({ events }: { events: CampaignEventItem[] }) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    if (events.length === 0) {
        return (
            <p className="text-sm text-muted-foreground">
                {t('campaigns.dashboard.noEvents')}
            </p>
        );
    }
    return (
        <ol
            className="space-y-3"
            aria-label={t('campaigns.dashboard.campaignEvents')}
        >
            {events.map((e) => {
                const detail = describeDetail(
                    t,
                    e.detail ?? {},
                    formatDateTime,
                );
                return (
                    <li
                        key={e.id}
                        className="text-sm"
                        data-testid="campaign-event"
                    >
                        <div className="font-medium">
                            {eventLabel(t, e.kind)}
                        </div>
                        <div className="text-xs text-muted-foreground">
                            {formatDateTime(e.created_at)} ·{' '}
                            {e.actor_id
                                ? t('campaigns.dashboard.byUser')
                                : t('campaigns.dashboard.byMessora')}
                        </div>
                        {detail && (
                            <div className="text-xs break-words">{detail}</div>
                        )}
                    </li>
                );
            })}
        </ol>
    );
}
