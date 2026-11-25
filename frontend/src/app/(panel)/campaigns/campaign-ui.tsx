import { Badge } from '@/components/ui/badge';
import { useT } from '@/i18n/client';
import type { Translate } from '@/i18n/translate';
import type { CampaignStatus } from '@/lib/types';
import { NUMBER_LOCALE } from '@/lib/format';
import { CAMPAIGN_STATUS_ICON, FallbackStatusIcon } from '@/lib/status-icons';
import { cn } from '@/lib/utils';
import { summarize, type Counts } from '@/lib/campaign-counts';
import { isConsentReason } from '@/lib/consent';
import type { Schemas } from '@/lib/types';

export const CAMPAIGN_STATUSES: CampaignStatus[] = [
    'draft',
    'scheduled',
    'preparing',
    'running',
    'paused',
    'paused_limit',
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
    paused_limit: 'secondary',
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

function StatusIcon({ status }: { status: string }) {
    const Icon = CAMPAIGN_STATUS_ICON[status] ?? FallbackStatusIcon;
    return (
        <Icon
            className={cn('size-3', status === 'preparing' && 'animate-spin')}
            aria-hidden
        />
    );
}

export function CampaignStatusBadge({ status }: { status: string }) {
    const t = useT();
    return (
        <Badge
            variant={isStatus(status) ? STATUS_VARIANT[status] : 'outline'}
            data-testid="campaign-status"
            data-status={status}
            className="gap-1"
        >
            <StatusIcon status={status} />
            {statusLabel(t, status)}
        </Badge>
    );
}

export { summarize, type Counts } from '@/lib/campaign-counts';

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
    const prev = a.previous_campaign as { campaign_id?: unknown } | undefined;
    if (prev && typeof prev.campaign_id === 'string')
        return t('campaigns.engine.audienceLabel', {
            name:
                names[prev.campaign_id] ??
                t('campaigns.engine.anotherCampaign'),
        });
    return '—';
}

/** Statuses in which a campaign can be resumed (every pause kind). */
export function isPaused(status: string): boolean {
    return status.startsWith('paused');
}

export const PRIORITIES = ['high', 'normal', 'low'] as const;
export type Priority = (typeof PRIORITIES)[number];

export function priorityLabel(t: Translate, p: string): string {
    const k = PRIORITIES.find((x) => x === p);
    return k ? t(`campaigns.engine.priorities.${k}`) : p;
}

const PAUSE_REASONS = [
    'manual',
    'system',
    'messaging_limit',
    'license_restricted',
    'offer_expired',
] as const;

/** Why a campaign was paused (unknown reasons are shown as they are). */
export function pauseReasonLabel(t: Translate, reason: string): string {
    const k = PAUSE_REASONS.find((x) => x === reason);
    return k
        ? t(`campaigns.engine.pauseReasons.${k}`)
        : reason.replaceAll('_', ' ');
}

/** Classes a previous-campaign audience may select. */
export const SELECTABLE_CLASSES = [
    'temporary',
    'rejected_by_meta',
    'ecosystem_limit',
    'undeliverable',
    'permanent',
    'never_sent',
] as const;
/** Classes that are hard blocks and never targeted again. */
export const BLOCKED_CLASSES = [
    'user_stopped_marketing',
    'blocked_by_consent',
] as const;
const ALL_CLASSES = [
    ...SELECTABLE_CLASSES,
    ...BLOCKED_CLASSES,
    'reached',
] as const;

/** Why a recipient was not reached. */
export function unreachedLabel(t: Translate, c: string): string {
    const k = ALL_CLASSES.find((x) => x === c);
    return k ? t(`campaigns.engine.classes.${k}`) : c;
}

/** A duration in seconds as "1 h 5 min", "3 min", "40 s". */
export function formatDuration(seconds: number): string {
    const s = Math.max(0, Math.round(seconds));
    if (s < 60) return `${s} s`;
    const m = Math.round(s / 60);
    if (m < 60) return `${m} min`;
    const h = Math.floor(m / 60);
    const rest = m % 60;
    if (h < 48) return rest ? `${h} h ${rest} min` : `${h} h`;
    return `${Math.round(h / 24)} d`;
}

/** A rate in messages per second, one decimal below 10. */
export function formatRate(r: number): string {
    return r < 10
        ? r.toLocaleString(NUMBER_LOCALE, { maximumFractionDigits: 1 })
        : Math.round(r).toLocaleString(NUMBER_LOCALE);
}

/** A recipient's skip reason (consent reasons are translated). */
export function skipReasonLabel(t: Translate, reason: string): string {
    return isConsentReason(reason)
        ? t(`consent.reasons.${reason}`)
        : reason.replaceAll('_', ' ');
}

const FAILURE_CLASSES = [
    'rejected_by_meta',
    'temporary',
    'permanent',
    'user_stopped_marketing',
    'undeliverable',
    'ecosystem_limit',
    'blocked_by_consent',
    'cancelled',
] as const;

/** Why a message ended failed, skipped or cancelled. */
export function failureClassLabel(
    t: Translate,
    c: NonNullable<Schemas['FailureClass']> | string,
): string {
    const known = FAILURE_CLASSES.find((x) => x === c);
    return known ? t(`consent.failureClasses.${known}`) : c;
}
