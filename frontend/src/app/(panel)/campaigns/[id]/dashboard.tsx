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
    RefreshCw,
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
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
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
import type {
    ActionResult,
    Campaign,
    CampaignEventItem,
    CampaignRecipient,
} from '@/lib/types';
import {
    controlCampaignAction,
    deleteCampaignAction,
    loadRecipientsAction,
    startCampaignAction,
} from '../actions';
import {
    audienceLabel,
    CampaignProgress,
    CampaignStatusBadge,
    summarize,
} from '../campaign-ui';
import { NUMBER_LOCALE } from '@/lib/format';
import { ClientDataTable } from '@/components/client-data-table';

type Perms = { edit: boolean; start: boolean; pause: boolean; del: boolean };

export function CampaignDashboard({
    campaign,
    events,
    recipients,
    recipientsCursor,
    sender,
    timezone,
    perms,
}: {
    campaign: Campaign;
    events: CampaignEventItem[];
    recipients: CampaignRecipient[];
    recipientsCursor?: string;
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
    const [live, setLive] = useState<Pick<
        Campaign,
        'status' | 'counts'
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

    // campaign.updated is published to campaign:all and campaign:<id> with
    // the same payload; the panel already listens on campaign:all.
    useRealtimeEvents((channel, e) => {
        if (
            e.type !== 'campaign.updated' ||
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
        setLive({ status, counts });
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
                <Controls campaign={c} perms={perms} />
            </div>

            {c.last_error && (
                <Alert variant="destructive">
                    <AlertTriangle />
                    <AlertTitle>
                        {c.status === 'paused'
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

            <div className="grid gap-6 lg:grid-cols-[1fr_20rem]">
                <Card className="min-w-0">
                    <CardHeader>
                        <CardTitle>
                            {t('campaigns.dashboard.recipients')}
                        </CardTitle>
                        <CardDescription>
                            {t('campaigns.dashboard.recipientsHint')}
                        </CardDescription>
                    </CardHeader>
                    <CardContent className="p-0">
                        <RecipientTable
                            campaignId={c.id}
                            initial={recipients}
                            initialCursor={recipientsCursor}
                        />
                    </CardContent>
                </Card>
                <Card>
                    <CardHeader>
                        <CardTitle>
                            {t('campaigns.dashboard.eventLog')}
                        </CardTitle>
                    </CardHeader>
                    <CardContent>
                        <EventLog events={events} />
                    </CardContent>
                </Card>
            </div>
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
}: {
    campaign: Campaign;
    perms: Perms;
}) {
    const t = useT();
    const router = useRouter();
    const { guard } = useReauth();
    const editable = c.status === 'draft' || c.status === 'scheduled';
    const active = c.status === 'running' || c.status === 'preparing';
    const deletable = !['running', 'paused', 'preparing'].includes(c.status);
    const control = (op: 'pause' | 'resume' | 'cancel') => async () => {
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
            {active && perms.pause && (
                <ConfirmButton
                    label={t('campaigns.dashboard.pause')}
                    icon={<Pause />}
                    title={t('campaigns.dashboard.pauseTitle')}
                    description={t('campaigns.dashboard.pauseDescription')}
                    confirm={t('campaigns.dashboard.pause')}
                    run={control('pause')}
                />
            )}
            {c.status === 'paused' && perms.start && (
                <ConfirmButton
                    variant="default"
                    label={t('campaigns.dashboard.resume')}
                    icon={<Play />}
                    title={t('campaigns.dashboard.resumeTitle')}
                    description={t('campaigns.dashboard.resumeDescription')}
                    confirm={t('campaigns.dashboard.resume')}
                    run={control('resume')}
                />
            )}
            {(active || c.status === 'paused' || c.status === 'scheduled') &&
                perms.pause && (
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

// ── Recipients ─────────────────────────────────────────────────────────────

const ALL = 'all';
const STATES = ['all', 'pending', 'queued', 'skipped', 'cancelled'] as const;

const MESSAGE_STATUSES = [
    'pending',
    'sending',
    'sent',
    'delivered',
    'read',
    'failed',
    'unknown',
    'held',
    'cancelled',
] as const;

function MessageStatus({ r }: { r: CampaignRecipient }) {
    const t = useT();
    if (r.state === 'skipped') {
        return (
            <Badge variant="outline">
                {r.skip_reason
                    ? t('campaigns.dashboard.skippedReason', {
                          reason: r.skip_reason.replaceAll('_', ' '),
                      })
                    : t('campaigns.dashboard.skipped')}
            </Badge>
        );
    }
    if (r.state === 'cancelled')
        return (
            <Badge variant="outline">
                {t('campaigns.dashboard.cancelled')}
            </Badge>
        );
    if (r.state === 'pending')
        return (
            <Badge variant="outline">{t('campaigns.dashboard.waiting')}</Badge>
        );
    const st: string = r.message_status ?? 'pending';
    const known = MESSAGE_STATUSES.find((x) => x === st);
    const stLabel = known
        ? t(`campaigns.dashboard.messageStatuses.${known}`)
        : st;
    const unknownHelp = t('campaigns.dashboard.unknownHelp');
    const variant =
        st === 'failed'
            ? 'destructive'
            : st === 'read' || st === 'delivered'
              ? 'secondary'
              : 'outline';
    const badge = <Badge variant={variant}>{stLabel}</Badge>;
    if (st !== 'unknown') return badge;
    return (
        <Tooltip>
            <TooltipTrigger asChild>
                <button
                    type="button"
                    className="rounded-full focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                    aria-label={`${stLabel}: ${unknownHelp}`}
                >
                    {badge}
                </button>
            </TooltipTrigger>
            <TooltipContent className="max-w-xs">{unknownHelp}</TooltipContent>
        </Tooltip>
    );
}

function RecipientTable({
    campaignId,
    initial,
    initialCursor,
}: {
    campaignId: string;
    initial: CampaignRecipient[];
    initialCursor?: string;
}) {
    const t = useT();
    const [state, setState] = useState<string>(ALL);
    const [items, setItems] = useState(initial);
    const [cursor, setCursor] = useState(initialCursor);
    const [error, setError] = useState<string | null>(null);
    const [pending, startTransition] = useTransition();

    const load = (nextState: string, after?: string) =>
        startTransition(async () => {
            const r = await loadRecipientsAction(
                campaignId,
                nextState === ALL ? '' : nextState,
                after,
            );
            if (!r.ok) {
                setError(r.message);
                return;
            }
            setError(null);
            setItems((l) => (after ? [...l, ...r.data.items] : r.data.items));
            setCursor(r.data.next);
        });

    return (
        <div className="space-y-3 px-4 pb-4">
            {error && (
                <p role="alert" className="text-sm text-destructive">
                    {error}
                </p>
            )}
            <ClientDataTable
                rows={items}
                rowId={(r) => r.id}
                storageKey="campaign-recipients"
                rowTestId="recipient-row"
                searchText={(r) =>
                    `${r.phone} ${r.error_code ?? ''} ${r.error_title ?? ''}`
                }
                searchPlaceholder={t('table.search')}
                toolbar={
                    <>
                        <Select
                            value={state}
                            onValueChange={(v) => {
                                setState(v);
                                load(v);
                            }}
                        >
                            <SelectTrigger
                                className="w-44"
                                aria-label={t(
                                    'campaigns.dashboard.recipientState',
                                )}
                            >
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {STATES.map((s) => (
                                    <SelectItem key={s} value={s}>
                                        {t(`campaigns.dashboard.states.${s}`)}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                        <Button
                            size="sm"
                            variant="ghost"
                            disabled={pending}
                            onClick={() => load(state)}
                        >
                            {pending ? <Spinner /> : <RefreshCw />}{' '}
                            {t('campaigns.dashboard.refresh')}
                        </Button>
                    </>
                }
                empty={
                    <p className="p-6 text-sm text-muted-foreground">
                        {state === ALL
                            ? t('campaigns.dashboard.noRecipients')
                            : t('campaigns.dashboard.noRecipientsInState')}
                    </p>
                }
                noMatch={
                    <p className="p-8 text-center text-sm text-muted-foreground">
                        {t('table.noMatch')}
                    </p>
                }
                columns={[
                    {
                        id: 'phone',
                        header: t('campaigns.dashboard.phone'),
                        hideable: false,
                        sortValue: (r) => r.phone,
                        cell: (r) => (
                            <span className="font-mono text-sm">{r.phone}</span>
                        ),
                    },
                    {
                        id: 'status',
                        header: t('campaigns.dashboard.status'),
                        sortValue: (r) =>
                            r.state === 'queued'
                                ? (r.message_status ?? 'pending')
                                : r.state,
                        cell: (r) => <MessageStatus r={r} />,
                    },
                    {
                        id: 'error',
                        header: t('campaigns.dashboard.error'),
                        sortValue: (r) => r.error_code ?? null,
                        cell: (r) => (
                            <div className="max-w-xs text-xs break-words">
                                {r.error_code ? (
                                    <>
                                        <code>{r.error_code}</code>
                                        {r.error_title
                                            ? ` ${r.error_title}`
                                            : ''}
                                    </>
                                ) : (
                                    <span className="text-muted-foreground">
                                        —
                                    </span>
                                )}
                            </div>
                        ),
                    },
                ]}
            />
            {cursor && (
                <div className="text-center">
                    <Button
                        variant="outline"
                        disabled={pending}
                        onClick={() => load(state, cursor)}
                    >
                        {pending && <Spinner />}{' '}
                        {t('campaigns.dashboard.loadMore')}
                    </Button>
                </div>
            )}
        </div>
    );
}
