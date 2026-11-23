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
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import {
    Tooltip,
    TooltipContent,
    TooltipTrigger,
} from '@/components/ui/tooltip';
import { formatDateTime } from '@/lib/format';
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

type Perms = { edit: boolean; start: boolean; pause: boolean; del: boolean };

const UNKNOWN_HELP =
    'Sent to Meta, but the reply was lost (timeout or dropped connection), so it is not known whether Meta accepted it. Messora never re-sends these automatically; Meta’s status webhook reconciles them to sent, delivered or read.';

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
                    <ArrowLeft className="size-4" /> Campaigns
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
                                ? 'Live'
                                : rt === 'connecting'
                                  ? 'Connecting…'
                                  : 'Offline'}
                        </Badge>
                    </div>
                    <p className="text-sm text-muted-foreground">
                        <span className="font-mono">
                            {c.template_name} ({c.template_language})
                        </span>
                        {' · '}
                        {audienceLabel(c.audience)}
                        {sender ? ` · from ${sender}` : ''}
                    </p>
                </div>
                <Controls campaign={c} perms={perms} />
            </div>

            {c.last_error && (
                <Alert variant="destructive">
                    <AlertTriangle />
                    <AlertTitle>
                        {c.status === 'paused'
                            ? 'Paused automatically'
                            : 'Last error'}
                    </AlertTitle>
                    <AlertDescription>{c.last_error}</AlertDescription>
                </Alert>
            )}

            <Card>
                <CardHeader>
                    <CardTitle>Progress</CardTitle>
                    <CardDescription>
                        {c.status === 'scheduled' && c.scheduled_at
                            ? `Starts ${formatInZone(c.scheduled_at, timezone)}.`
                            : notStarted
                              ? 'Recipients are collected when the campaign starts.'
                              : c.finished_at
                                ? `Finished ${formatDateTime(c.finished_at)}.`
                                : c.started_at
                                  ? `Started ${formatDateTime(c.started_at)}.`
                                  : 'Collecting recipients…'}
                    </CardDescription>
                </CardHeader>
                <CardContent className="space-y-6">
                    <CampaignProgress
                        counts={c.counts}
                        total={c.total_recipients}
                        label="Campaign progress"
                    />
                    <dl
                        className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-6"
                        data-testid="campaign-counts"
                    >
                        <Stat label="Recipients" value={s.recipients} />
                        <Stat label="Skipped" value={s.skipped} />
                        <Stat label="Waiting" value={s.waiting} />
                        <Stat label="Sending" value={s.inFlight} />
                        <Stat label="Sent" value={s.sent} />
                        <Stat label="Delivered" value={s.delivered} />
                        <Stat label="Read" value={s.read} />
                        <Stat label="Failed" value={s.failed} tone="bad" />
                        <Stat
                            label="Unknown"
                            value={s.unknown}
                            tone="warn"
                            help={UNKNOWN_HELP}
                        />
                        <Stat
                            label="Held"
                            value={s.held}
                            help="Claimed while the campaign was paused; sent after resume."
                        />
                        <Stat label="Cancelled" value={s.cancelled} />
                    </dl>
                    <p className="text-xs text-muted-foreground">
                        Window: {c.window_size || 'automatic'} messages in
                        flight · created {formatDateTime(c.created_at)}
                    </p>
                </CardContent>
            </Card>

            <div className="grid gap-6 lg:grid-cols-[1fr_20rem]">
                <Card className="min-w-0">
                    <CardHeader>
                        <CardTitle>Recipients</CardTitle>
                        <CardDescription>
                            Each contact once; opted-out and duplicate numbers
                            are skipped.
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
                        <CardTitle>Event log</CardTitle>
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
    label,
    value,
    tone,
    help,
}: {
    label: string;
    value: number;
    tone?: 'bad' | 'warn';
    help?: string;
}) {
    return (
        <div
            className="rounded-lg border p-3"
            data-testid={`count-${label.toLowerCase()}`}
        >
            <dt className="flex items-center gap-1 text-xs text-muted-foreground">
                {label}
                {help && (
                    <Tooltip>
                        <TooltipTrigger asChild>
                            <button
                                type="button"
                                className="rounded-full text-muted-foreground hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                                aria-label={`What does “${label}” mean?`}
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
                {value.toLocaleString()}
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
                        Keep as is
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
                        <Pencil /> Edit
                    </Link>
                </Button>
            )}
            {editable && perms.start && (
                <ConfirmButton
                    variant="default"
                    label="Start now"
                    icon={<Play />}
                    title="Start sending now?"
                    description="Recipients are collected and messages go out right away. You can pause or cancel while it runs."
                    confirm="Start now"
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
                    label="Pause"
                    icon={<Pause />}
                    title="Pause this campaign?"
                    description="No new messages are sent. Messages already handed to Meta still arrive; unsent ones are held until you resume."
                    confirm="Pause"
                    run={control('pause')}
                />
            )}
            {c.status === 'paused' && perms.start && (
                <ConfirmButton
                    variant="default"
                    label="Resume"
                    icon={<Play />}
                    title="Resume sending?"
                    description="Held messages are queued again and the remaining recipients are sent to."
                    confirm="Resume"
                    run={control('resume')}
                />
            )}
            {(active || c.status === 'paused' || c.status === 'scheduled') &&
                perms.pause && (
                    <ConfirmButton
                        destructive
                        label={
                            c.status === 'scheduled'
                                ? 'Cancel schedule'
                                : 'Cancel'
                        }
                        icon={<Square />}
                        title="Cancel this campaign for good?"
                        description="Recipients who have not been sent to yet are cancelled. This cannot be undone; messages already sent are not affected."
                        confirm="Cancel campaign"
                        run={control('cancel')}
                    />
                )}
            {deletable && perms.del && (
                <ConfirmButton
                    destructive
                    label="Delete"
                    icon={<Trash2 />}
                    title="Delete this campaign?"
                    description="The campaign, its recipients and its event log are removed. Sent messages stay in conversations."
                    confirm="Delete"
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

const EVENT_LABEL: Record<string, string> = {
    scheduled: 'Scheduled',
    started: 'Started',
    paused: 'Paused',
    auto_paused: 'Paused automatically',
    resumed: 'Resumed',
    cancelled: 'Cancelled',
    completed: 'Completed',
    failed: 'Failed',
};

function describeDetail(d: Record<string, unknown>): string {
    const parts: string[] = [];
    if (typeof d.reason === 'string') parts.push(d.reason);
    if (typeof d.error === 'string') parts.push(d.error);
    if (typeof d.at === 'string') parts.push(`for ${formatDateTime(d.at)}`);
    if (d.scheduled === true) parts.push('on schedule');
    return parts.join(' · ');
}

function EventLog({ events }: { events: CampaignEventItem[] }) {
    if (events.length === 0) {
        return (
            <p className="text-sm text-muted-foreground">
                Nothing has happened yet.
            </p>
        );
    }
    return (
        <ol className="space-y-3" aria-label="Campaign events">
            {events.map((e) => {
                const detail = describeDetail(e.detail ?? {});
                return (
                    <li
                        key={e.id}
                        className="text-sm"
                        data-testid="campaign-event"
                    >
                        <div className="font-medium">
                            {EVENT_LABEL[e.kind] ?? e.kind}
                        </div>
                        <div className="text-xs text-muted-foreground">
                            {formatDateTime(e.created_at)} ·{' '}
                            {e.actor_id ? 'by a user' : 'by Messora'}
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
const STATES = [
    { value: ALL, label: 'All states' },
    { value: 'pending', label: 'Waiting' },
    { value: 'queued', label: 'Queued / sent' },
    { value: 'skipped', label: 'Skipped' },
    { value: 'cancelled', label: 'Cancelled' },
];

function MessageStatus({ r }: { r: CampaignRecipient }) {
    if (r.state === 'skipped') {
        return (
            <Badge variant="outline">
                Skipped
                {r.skip_reason ? `: ${r.skip_reason.replaceAll('_', ' ')}` : ''}
            </Badge>
        );
    }
    if (r.state === 'cancelled')
        return <Badge variant="outline">Cancelled</Badge>;
    if (r.state === 'pending') return <Badge variant="outline">Waiting</Badge>;
    const st = r.message_status ?? 'pending';
    const variant =
        st === 'failed'
            ? 'destructive'
            : st === 'read' || st === 'delivered'
              ? 'secondary'
              : 'outline';
    const badge = <Badge variant={variant}>{st}</Badge>;
    if (st !== 'unknown') return badge;
    return (
        <Tooltip>
            <TooltipTrigger asChild>
                <button
                    type="button"
                    className="rounded-full focus-visible:ring-2 focus-visible:ring-ring focus-visible:outline-none"
                    aria-label={`unknown: ${UNKNOWN_HELP}`}
                >
                    {badge}
                </button>
            </TooltipTrigger>
            <TooltipContent className="max-w-xs">{UNKNOWN_HELP}</TooltipContent>
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
    const [state, setState] = useState(ALL);
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
        <div>
            <div className="flex flex-wrap items-center gap-2 border-b px-4 pb-3">
                <Select
                    value={state}
                    onValueChange={(v) => {
                        setState(v);
                        load(v);
                    }}
                >
                    <SelectTrigger
                        className="w-44"
                        aria-label="Recipient state"
                    >
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        {STATES.map((s) => (
                            <SelectItem key={s.value} value={s.value}>
                                {s.label}
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
                    {pending ? <Spinner /> : <RefreshCw />} Refresh
                </Button>
            </div>
            {error && (
                <p role="alert" className="px-4 py-3 text-sm text-destructive">
                    {error}
                </p>
            )}
            {items.length === 0 ? (
                <p className="p-6 text-sm text-muted-foreground">
                    {state === ALL
                        ? 'No recipients yet. They are collected when the campaign starts.'
                        : 'No recipients in this state.'}
                </p>
            ) : (
                <Table>
                    <TableHeader>
                        <TableRow>
                            <TableHead>Phone</TableHead>
                            <TableHead>Status</TableHead>
                            <TableHead>Error</TableHead>
                        </TableRow>
                    </TableHeader>
                    <TableBody>
                        {items.map((r) => (
                            <TableRow key={r.id} data-testid="recipient-row">
                                <TableCell className="font-mono text-sm">
                                    {r.phone}
                                </TableCell>
                                <TableCell>
                                    <MessageStatus r={r} />
                                </TableCell>
                                <TableCell className="max-w-xs text-xs break-words">
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
                                </TableCell>
                            </TableRow>
                        ))}
                    </TableBody>
                </Table>
            )}
            {cursor && (
                <div className="border-t p-3 text-center">
                    <Button
                        variant="outline"
                        disabled={pending}
                        onClick={() => load(state, cursor)}
                    >
                        {pending && <Spinner />} Load more
                    </Button>
                </div>
            )}
        </div>
    );
}
