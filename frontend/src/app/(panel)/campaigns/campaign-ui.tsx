import { Badge } from '@/components/ui/badge';
import { useT } from '@/i18n/client';
import type { Translate } from '@/i18n/translate';
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

const STATUS_VARIANT: Record<
    CampaignStatus,
    'default' | 'secondary' | 'destructive' | 'outline'
> = {
    draft: 'outline',
    scheduled: 'outline',
    preparing: 'default',
    running: 'default',
    paused: 'secondary',
    completed: 'secondary',
    cancelled: 'outline',
    failed: 'destructive',
};

function isStatus(s: string): s is CampaignStatus {
    return s in STATUS_VARIANT;
}

export function statusLabel(t: Translate, s: string): string {
    return isStatus(s) ? t(`campaigns.statuses.${s}`) : s;
}

export function CampaignStatusBadge({ status }: { status: string }) {
    const t = useT();
    return (
        <Badge
            variant={isStatus(status) ? STATUS_VARIANT[status] : 'outline'}
            data-testid="campaign-status"
            data-status={status}
        >
            {statusLabel(t, status)}
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
    { key: 'read', className: 'bg-emerald-600' },
    { key: 'delivered', className: 'bg-emerald-400' },
    { key: 'sent', className: 'bg-sky-500' },
    { key: 'unknown', className: 'bg-amber-400' },
    { key: 'failed', className: 'bg-destructive' },
    { key: 'skipped', className: 'bg-muted-foreground/40' },
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
    const t = useT();
    const s = summarize(counts, total);
    return (
        <div className="space-y-1">
            <div
                role="progressbar"
                aria-label={label}
                aria-valuemin={0}
                aria-valuemax={100}
                aria-valuenow={s.percent}
                aria-valuetext={t('campaigns.progressValue', {
                    percent: s.percent,
                    done: s.done,
                    total: s.recipients,
                })}
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
                                title={`${t(`campaigns.counts.${seg.key}`)}: ${v}`}
                            />
                        );
                    })}
            </div>
            <div className="flex justify-between text-xs text-muted-foreground">
                <span>
                    {t('campaigns.processed', {
                        done: s.done.toLocaleString(NUMBER_LOCALE),
                        total: s.recipients.toLocaleString(NUMBER_LOCALE),
                    })}
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
                            {t(`campaigns.counts.${seg.key}`)}
                        </li>
                    ))}
                </ul>
            )}
        </div>
    );
}

/** Describes an audience for lists and the review step. */
export function audienceLabel(
    t: Translate,
    a: Record<string, unknown>,
    names: Record<string, string> = {},
): string {
    if (a.all) return t('campaigns.audience.all');
    if (typeof a.segment_id === 'string')
        return t('campaigns.audience.segment', {
            name: names[a.segment_id] ?? t('campaigns.audience.savedSegment'),
        });
    if (typeof a.group_id === 'string')
        return t('campaigns.audience.group', {
            name: names[a.group_id] ?? t('campaigns.audience.contactGroup'),
        });
    if (typeof a.tag_id === 'string')
        return t('campaigns.audience.tag', {
            name: names[a.tag_id] ?? t('campaigns.audience.someTag'),
        });
    return '—';
}
