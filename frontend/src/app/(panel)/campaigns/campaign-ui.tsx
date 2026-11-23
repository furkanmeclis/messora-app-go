import { Badge } from '@/components/ui/badge';
import type { CampaignStatus } from '@/lib/types';
import { NUMBER_LOCALE } from '@/lib/format';

export const CAMPAIGN_STATUSES: CampaignStatus[] = [
    'draft',
    'scheduled',
    'preparing',
    'running',
    'paused',
    'completed',
    'cancelled',
    'failed',
];

const STATUS: Record<
    CampaignStatus,
    {
        label: string;
        variant: 'default' | 'secondary' | 'destructive' | 'outline';
    }
> = {
    draft: { label: 'Draft', variant: 'outline' },
    scheduled: { label: 'Scheduled', variant: 'outline' },
    preparing: { label: 'Preparing', variant: 'default' },
    running: { label: 'Running', variant: 'default' },
    paused: { label: 'Paused', variant: 'secondary' },
    completed: { label: 'Completed', variant: 'secondary' },
    cancelled: { label: 'Cancelled', variant: 'outline' },
    failed: { label: 'Failed', variant: 'destructive' },
};

export function statusLabel(s: string): string {
    return STATUS[s as CampaignStatus]?.label ?? s;
}

export function CampaignStatusBadge({ status }: { status: string }) {
    const s = STATUS[status as CampaignStatus] ?? {
        label: status,
        variant: 'outline' as const,
    };
    return (
        <Badge
            variant={s.variant}
            data-testid="campaign-status"
            data-status={status}
        >
            {s.label}
        </Badge>
    );
}

export type Counts = Record<string, number>;

/** Aggregates the API's counters (recipients_* and message statuses). */
export function summarize(counts: Counts, total: number) {
    const n = (k: string) => counts[k] ?? 0;
    const recipients = Math.max(n('recipients'), total);
    const skipped = n('recipients_skipped');
    const cancelled = n('recipients_cancelled') + n('cancelled');
    const delivered = n('delivered');
    const read = n('read');
    const sent = n('sent');
    const failed = n('failed');
    const unknown = n('unknown');
    const held = n('held');
    const inFlight = n('pending') + n('sending');
    const waiting = n('recipients_pending');
    const done =
        skipped +
        n('recipients_cancelled') +
        sent +
        delivered +
        read +
        failed +
        unknown +
        n('cancelled');
    return {
        recipients,
        skipped,
        cancelled,
        sent,
        delivered,
        read,
        failed,
        unknown,
        held,
        inFlight,
        waiting,
        done: Math.min(done, recipients),
        percent:
            recipients > 0
                ? Math.min(100, Math.round((done / recipients) * 100))
                : 0,
    };
}

const SEGMENTS = [
    { key: 'read', label: 'Read', className: 'bg-emerald-600' },
    { key: 'delivered', label: 'Delivered', className: 'bg-emerald-400' },
    { key: 'sent', label: 'Sent', className: 'bg-sky-500' },
    { key: 'unknown', label: 'Unknown', className: 'bg-amber-400' },
    { key: 'failed', label: 'Failed', className: 'bg-destructive' },
    { key: 'skipped', label: 'Skipped', className: 'bg-muted-foreground/40' },
] as const;

/** Stacked bar of what happened to the recipients so far. */
export function CampaignProgress({
    counts,
    total,
    compact,
    label,
}: {
    counts: Counts;
    total: number;
    compact?: boolean;
    label: string;
}) {
    const s = summarize(counts, total);
    return (
        <div className="space-y-1">
            <div
                role="progressbar"
                aria-label={label}
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={s.percent}
                aria-valuetext={`${s.percent}% processed (${s.done} of ${s.recipients})`}
                className={`flex w-full overflow-hidden rounded-full bg-muted ${compact ? 'h-1.5' : 'h-3'}`}
            >
                {s.recipients > 0 &&
                    SEGMENTS.map((seg) => {
                        const v = s[seg.key];
                        if (!v) return null;
                        return (
                            <div
                                key={seg.key}
                                className={seg.className}
                                style={{
                                    width: `${(v / s.recipients) * 100}%`,
                                }}
                                title={`${seg.label}: ${v}`}
                            />
                        );
                    })}
            </div>
            <div className="flex justify-between text-xs text-muted-foreground">
                <span>
                    {s.done.toLocaleString(NUMBER_LOCALE)} /{' '}
                    {s.recipients.toLocaleString(NUMBER_LOCALE)} processed
                </span>
                <span>{s.percent}%</span>
            </div>
            {!compact && (
                <ul className="flex flex-wrap gap-x-3 gap-y-1 text-xs text-muted-foreground">
                    {SEGMENTS.map((seg) => (
                        <li key={seg.key} className="flex items-center gap-1">
                            <span
                                aria-hidden
                                className={`inline-block size-2 rounded-full ${seg.className}`}
                            />
                            {seg.label}
                        </li>
                    ))}
                </ul>
            )}
        </div>
    );
}

/** Describes an audience for lists and the review step. */
export function audienceLabel(
    a: Record<string, unknown>,
    names: Record<string, string> = {},
): string {
    if (a.all) return 'All contacts';
    if (typeof a.segment_id === 'string')
        return `Segment: ${names[a.segment_id] ?? 'saved segment'}`;
    if (typeof a.group_id === 'string')
        return `Group: ${names[a.group_id] ?? 'contact group'}`;
    if (typeof a.tag_id === 'string') return `Tag: ${names[a.tag_id] ?? 'tag'}`;
    return '—';
}
