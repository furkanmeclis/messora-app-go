'use client';

import { useEffect, useState, useTransition } from 'react';
import { useRouter } from 'next/navigation';
import { Hourglass, PauseCircle, Play } from 'lucide-react';
import { notify } from '@/components/notify';
import { useReauth } from '@/components/reauth';
import { ClientDataTable } from '@/components/client-data-table';
import { useFormatDateTime } from '@/components/display-preferences';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { DateTimePicker } from '@/components/ui/date-picker';
import {
    Dialog,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from '@/components/ui/dialog';
import { Field, FieldLabel } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { NumberField } from '@/components/ui/number-field';
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import { Switch } from '@/components/ui/switch';
import { useT } from '@/i18n/client';
import { NUMBER_LOCALE } from '@/lib/format';
import { zonedToUtc } from '@/lib/timezone';
import type { ActionResult, Campaign } from '@/lib/types';
import {
    resumeCampaignAction,
    resumePreviewAction,
    updateCampaignSettingsAction,
    type CampaignPause,
    type CampaignSettings,
    type ResumePreview,
} from '../actions';
import {
    formatDuration,
    formatRate,
    isPaused,
    pauseReasonLabel,
    PRIORITIES,
    skipReasonLabel,
    priorityLabel,
    type Priority,
} from '../campaign-ui';

const fmt = (n: number) => n.toLocaleString(NUMBER_LOCALE);

/** Why the campaign is paused and when it resumes by itself. */
/** Why campaign protection stopped a campaign, from its pause detail. */
export function spamRuleText(
    t: ReturnType<typeof useT>,
    detail: Record<string, unknown> | undefined,
): string | null {
    const rule = typeof detail?.rule === 'string' ? detail.rule : '';
    const s = (v: unknown) => (v == null ? '' : String(v));
    switch (rule) {
        case 'error_131048':
            return t('analytics.spam.rules.error_131048');
        case 'account_restriction':
            return t('analytics.spam.rules.account_restriction', {
                event: s(detail?.event),
            });
        case 'ecosystem_rate':
        case 'opt_out_rate':
            return t(`analytics.spam.rules.${rule}`, {
                rate: s(detail?.rate_pct),
                attempts: s(detail?.attempts),
                threshold: s(detail?.threshold_pct),
            });
        case 'number_quality':
        case 'template_quality':
            return t(`analytics.spam.rules.${rule}`, {
                quality: s(detail?.quality),
            });
        case 'template_paused':
            return t('analytics.spam.rules.template_paused', {
                status: s(detail?.status),
            });
    }
    return null;
}

export function PauseNotice({
    campaign: c,
    pauses,
}: {
    campaign: Campaign;
    pauses?: CampaignPause[];
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    if (!isPaused(c.status) || !c.paused_reason) return null;
    const limit = c.status === 'paused_limit';
    const spam = c.paused_reason === 'spam_protection';
    const open = pauses?.find((p) => !p.resumed_at);
    const why = spam ? spamRuleText(t, open?.detail) : null;
    return (
        <Alert data-testid="pause-notice" data-reason={c.paused_reason}>
            {limit ? <Hourglass /> : <PauseCircle />}
            <AlertTitle>
                {limit
                    ? t('campaigns.engine.pausedLimitTitle')
                    : t('campaigns.engine.pausedReasonTitle', {
                          reason: pauseReasonLabel(t, c.paused_reason),
                      })}
            </AlertTitle>
            <AlertDescription>
                {limit && <p>{t('campaigns.engine.pausedLimitBody')}</p>}
                {spam && (
                    <p data-testid="spam-reason">
                        {why ?? t('analytics.spam.title')}
                    </p>
                )}
                {c.resume_at && (
                    <p data-testid="resume-at">
                        {t('campaigns.engine.resumesAt', {
                            date: formatDateTime(c.resume_at),
                        })}
                    </p>
                )}
            </AlertDescription>
        </Alert>
    );
}

/** Priority (editable while the campaign is in progress), rate and ETA. */
export function EngineSummary({
    campaign: c,
    canChange,
}: {
    campaign: Campaign;
    canChange: boolean;
}) {
    const t = useT();
    const router = useRouter();
    const { guard } = useReauth();
    const [pending, startTransition] = useTransition();
    const finished = ['completed', 'cancelled', 'failed'].includes(c.status);
    const change = (p: Priority) =>
        startTransition(async () => {
            const r = await guard(() =>
                updateCampaignSettingsAction(c.id, { priority: p }),
            );
            if (notify(r)) router.refresh();
        });
    return (
        <dl
            className="flex flex-wrap items-center gap-x-6 gap-y-2 text-sm"
            data-testid="campaign-engine"
        >
            <div className="flex items-center gap-2">
                <dt className="text-muted-foreground">
                    {t('campaigns.engine.priority')}
                </dt>
                <dd>
                    {canChange && !finished ? (
                        <Select
                            value={c.priority}
                            disabled={pending}
                            onValueChange={(v) => change(v as Priority)}
                        >
                            <SelectTrigger
                                size="sm"
                                className="w-32"
                                aria-label={t(
                                    'campaigns.engine.changePriority',
                                )}
                                data-testid="priority-select"
                            >
                                <SelectValue />
                            </SelectTrigger>
                            <SelectContent>
                                {PRIORITIES.map((p) => (
                                    <SelectItem key={p} value={p}>
                                        {priorityLabel(t, p)}
                                    </SelectItem>
                                ))}
                            </SelectContent>
                        </Select>
                    ) : (
                        <span data-testid="priority-value">
                            {priorityLabel(t, c.priority)}
                        </span>
                    )}
                </dd>
            </div>
            {c.max_mps != null && (
                <div className="flex items-center gap-2">
                    <dt className="text-muted-foreground">
                        {t('campaigns.engine.maxMps')}
                    </dt>
                    <dd className="tabular-nums">{formatRate(c.max_mps)}</dd>
                </div>
            )}
            <div className="flex items-center gap-2">
                <dt className="text-muted-foreground">
                    {t('campaigns.engine.rate')}
                </dt>
                <dd className="tabular-nums" data-testid="send-rate">
                    {c.send_rate != null && c.status === 'running'
                        ? t('campaigns.engine.ratePerSecond', {
                              rate: formatRate(c.send_rate),
                          })
                        : t('campaigns.engine.unknown')}
                </dd>
            </div>
            <div className="flex items-center gap-2">
                <dt className="text-muted-foreground">
                    {t('campaigns.engine.eta')}
                </dt>
                <dd className="tabular-nums" data-testid="campaign-eta">
                    {c.eta_seconds != null
                        ? t('campaigns.engine.etaValue', {
                              value: formatDuration(c.eta_seconds),
                          })
                        : t('campaigns.engine.unknown')}
                </dd>
            </div>
        </dl>
    );
}

/**
 * Resume with a pre-resume summary: remaining recipients with consent
 * checked again, the rate the campaign gets and the estimated time; name,
 * priority, rate cap and a later start can be changed.
 */
export function ResumeDialog({
    campaign: c,
    timezone,
}: {
    campaign: Campaign;
    timezone: string;
}) {
    const t = useT();
    const router = useRouter();
    const formatDateTime = useFormatDateTime();
    const { guard } = useReauth();
    const [open, setOpen] = useState(false);
    const [name, setName] = useState(c.name);
    const [priority, setPriority] = useState<Priority>(c.priority);
    const [maxMps, setMaxMps] = useState<number | null>(c.max_mps ?? null);
    const [later, setLater] = useState(false);
    const [at, setAt] = useState('');
    const [preview, setPreview] = useState<
        ActionResult<ResumePreview> | undefined
    >();
    const [pending, startTransition] = useTransition();

    const settings: CampaignSettings = {
        ...(name.trim() && name.trim() !== c.name ? { name: name.trim() } : {}),
        ...(priority !== c.priority ? { priority } : {}),
        ...(maxMps
            ? maxMps !== c.max_mps
                ? { max_mps: maxMps }
                : {}
            : c.max_mps
              ? { clear_max_mps: true }
              : {}),
    };
    const key = JSON.stringify(settings);

    useEffect(() => {
        if (!open) return;
        let cancelled = false;
        resumePreviewAction(c.id, JSON.parse(key) as CampaignSettings).then(
            (r) => {
                if (!cancelled) setPreview(r);
            },
        );
        return () => {
            cancelled = true;
        };
    }, [open, key, c.id]);

    const resumeAt = later ? zonedToUtc(at, timezone) : null;
    const submit = () =>
        startTransition(async () => {
            const r = await guard(() =>
                resumeCampaignAction(c.id, {
                    ...settings,
                    ...(resumeAt ? { resume_at: resumeAt.toISOString() } : {}),
                }),
            );
            if (notify(r)) {
                setOpen(false);
                router.refresh();
            }
        });

    const p = preview?.ok ? preview.data : null;
    return (
        <Dialog
            open={open}
            onOpenChange={(o) => {
                setOpen(o);
                if (o) setPreview(undefined);
            }}
        >
            <DialogTrigger asChild>
                <Button data-testid="resume-open">
                    <Play /> {t('campaigns.dashboard.resume')}
                </Button>
            </DialogTrigger>
            <DialogContent className="sm:max-w-lg">
                <DialogHeader>
                    <DialogTitle>
                        {t('campaigns.engine.preview.title')}
                    </DialogTitle>
                    <DialogDescription>
                        {t('campaigns.engine.preview.neverResent')}
                    </DialogDescription>
                </DialogHeader>
                <div className="space-y-4" data-testid="resume-preview">
                    {c.paused_reason === 'spam_protection' && (
                        <Alert
                            variant="destructive"
                            data-testid="spam-resume-warning"
                        >
                            <AlertDescription>
                                {t('analytics.spam.resumeWarning')}
                            </AlertDescription>
                        </Alert>
                    )}
                    {!preview ? (
                        <p
                            className="flex items-center gap-2 text-sm text-muted-foreground"
                            role="status"
                        >
                            <Spinner /> {t('campaigns.engine.preview.loading')}
                        </p>
                    ) : !preview.ok ? (
                        <p role="alert" className="text-sm text-destructive">
                            {preview.message}
                        </p>
                    ) : (
                        p && (
                            <dl className="grid grid-cols-[1fr_auto] gap-x-4 gap-y-1 text-sm">
                                <dt className="text-muted-foreground">
                                    {t('campaigns.engine.preview.remaining')}
                                </dt>
                                <dd
                                    className="text-right tabular-nums"
                                    data-testid="preview-remaining"
                                >
                                    {fmt(p.remaining)}
                                </dd>
                                <dt className="text-muted-foreground">
                                    {t('campaigns.engine.preview.eligible')}
                                </dt>
                                <dd
                                    className="text-right tabular-nums"
                                    data-testid="preview-eligible"
                                >
                                    {fmt(p.eligible)}
                                </dd>
                                {p.blocked > 0 && (
                                    <>
                                        <dt className="text-muted-foreground">
                                            {t(
                                                'campaigns.engine.preview.blocked',
                                            )}
                                            {p.blocked_reasons.length > 0 &&
                                                ` (${p.blocked_reasons
                                                    .map(
                                                        (r) =>
                                                            `${skipReasonLabel(t, r.reason)}: ${fmt(r.count)}`,
                                                    )
                                                    .join(', ')})`}
                                        </dt>
                                        <dd className="text-right text-destructive tabular-nums">
                                            {fmt(p.blocked)}
                                        </dd>
                                    </>
                                )}
                                <dt className="text-muted-foreground">
                                    {t('campaigns.engine.preview.share')}
                                </dt>
                                <dd className="text-right tabular-nums">
                                    {t('campaigns.engine.ratePerSecond', {
                                        rate: formatRate(p.share_mps),
                                    })}
                                </dd>
                                <dt className="text-muted-foreground">
                                    {t('campaigns.engine.preview.estimate')}
                                </dt>
                                <dd
                                    className="text-right tabular-nums"
                                    data-testid="preview-estimate"
                                >
                                    {p.estimated_seconds != null
                                        ? t('campaigns.engine.etaValue', {
                                              value: formatDuration(
                                                  p.estimated_seconds,
                                              ),
                                          })
                                        : t('campaigns.engine.unknown')}
                                </dd>
                                <dt className="text-muted-foreground">
                                    {t('campaigns.engine.preview.limitLeft')}
                                </dt>
                                <dd className="text-right tabular-nums">
                                    {p.limit.limit > 0
                                        ? `${fmt(Math.max(p.limit.limit - p.limit.used - p.limit.in_flight, 0))} / ${fmt(p.limit.limit)}${
                                              p.limit.frees_at
                                                  ? ` · ${t('campaigns.engine.preview.limitFrees', { date: formatDateTime(p.limit.frees_at) })}`
                                                  : ''
                                          }`
                                        : t(
                                              'campaigns.engine.preview.limitNone',
                                          )}
                                </dd>
                            </dl>
                        )
                    )}
                    <Field>
                        <FieldLabel htmlFor="rs-name">
                            {t('campaigns.engine.preview.name')}
                        </FieldLabel>
                        <Input
                            id="rs-name"
                            value={name}
                            maxLength={120}
                            onChange={(e) => setName(e.target.value)}
                        />
                    </Field>
                    <div className="grid gap-4 sm:grid-cols-2">
                        <Field>
                            <FieldLabel htmlFor="rs-priority">
                                {t('campaigns.engine.priority')}
                            </FieldLabel>
                            <Select
                                value={priority}
                                onValueChange={(v) =>
                                    setPriority(v as Priority)
                                }
                            >
                                <SelectTrigger
                                    id="rs-priority"
                                    className="w-full"
                                    data-testid="resume-priority"
                                >
                                    <SelectValue />
                                </SelectTrigger>
                                <SelectContent>
                                    {PRIORITIES.map((x) => (
                                        <SelectItem key={x} value={x}>
                                            {priorityLabel(t, x)}
                                        </SelectItem>
                                    ))}
                                </SelectContent>
                            </Select>
                        </Field>
                        <Field>
                            <FieldLabel htmlFor="rs-maxmps">
                                {t('campaigns.engine.maxMps')}
                            </FieldLabel>
                            <NumberField
                                id="rs-maxmps"
                                min={0}
                                max={1000}
                                step={0.1}
                                allowEmpty
                                placeholder={t('campaigns.engine.noCap')}
                                value={maxMps}
                                onValueChange={(v) =>
                                    setMaxMps(v === 0 ? null : v)
                                }
                            />
                        </Field>
                    </div>
                    <div className="flex items-center gap-2">
                        <Switch
                            id="rs-later"
                            checked={later}
                            onCheckedChange={setLater}
                        />
                        <label htmlFor="rs-later" className="text-sm">
                            {t('campaigns.engine.preview.resumeLater')}
                        </label>
                    </div>
                    {later && (
                        <Field>
                            <FieldLabel htmlFor="rs-at">
                                {t('campaigns.engine.preview.resumeAt', {
                                    zone: timezone,
                                })}
                            </FieldLabel>
                            <DateTimePicker
                                id="rs-at"
                                value={at}
                                onValueChange={setAt}
                            />
                        </Field>
                    )}
                </div>
                <DialogFooter>
                    <Button
                        variant="outline"
                        disabled={pending}
                        onClick={() => setOpen(false)}
                    >
                        {t('campaigns.dashboard.keepAsIs')}
                    </Button>
                    <Button
                        disabled={pending || (later && !resumeAt)}
                        onClick={submit}
                        data-testid="resume-confirm"
                    >
                        {pending && <Spinner />}{' '}
                        {later
                            ? t('campaigns.engine.preview.schedule')
                            : t('campaigns.engine.preview.confirm')}
                    </Button>
                </DialogFooter>
            </DialogContent>
        </Dialog>
    );
}

function describeChanges(changes: Record<string, unknown> | null | undefined) {
    if (!changes) return '';
    return Object.entries(changes)
        .map(([k, v]) =>
            Array.isArray(v) ? `${k}: ${String(v[0])} → ${String(v[1])}` : k,
        )
        .join(', ');
}

/** Every pause with the counts at that moment. */
export function PauseHistory({ pauses }: { pauses: CampaignPause[] }) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    return (
        <section className="space-y-3" aria-labelledby="pause-history">
            <h2 id="pause-history" className="text-lg font-semibold">
                {t('campaigns.engine.pauses')}
            </h2>
            <ClientDataTable
                rows={pauses}
                rowId={(p) => p.id}
                storageKey="campaign-pauses"
                rowTestId="pause-row"
                empty={
                    <p className="p-6 text-sm text-muted-foreground">
                        {t('campaigns.engine.noPauses')}
                    </p>
                }
                columns={[
                    {
                        id: 'paused_at',
                        header: t('campaigns.engine.pausedAt'),
                        hideable: false,
                        sortValue: (p) => p.paused_at,
                        cell: (p) => (
                            <span className="whitespace-nowrap">
                                {formatDateTime(p.paused_at)}
                            </span>
                        ),
                    },
                    {
                        id: 'reason',
                        header: t('campaigns.engine.reason'),
                        sortValue: (p) => p.reason,
                        cell: (p) => pauseReasonLabel(t, p.reason),
                    },
                    {
                        id: 'snapshot',
                        header: t('campaigns.engine.snapshot'),
                        cell: (p) => (
                            <span className="text-xs text-muted-foreground tabular-nums">
                                {t('campaigns.engine.snapshotValue', {
                                    sent: fmt(
                                        (p.snapshot.sent ?? 0) +
                                            (p.snapshot.delivered ?? 0) +
                                            (p.snapshot.read ?? 0),
                                    ),
                                    failed: fmt(p.snapshot.failed ?? 0),
                                    waiting: fmt(
                                        (p.snapshot.recipients_pending ?? 0) +
                                            (p.snapshot.held ?? 0),
                                    ),
                                })}
                            </span>
                        ),
                    },
                    {
                        id: 'resume_at',
                        header: t('campaigns.engine.resumeAt'),
                        sortValue: (p) => p.resume_at ?? null,
                        cell: (p) =>
                            p.resume_at ? formatDateTime(p.resume_at) : '—',
                    },
                    {
                        id: 'resumed_at',
                        header: t('campaigns.engine.resumedAt'),
                        sortValue: (p) => p.resumed_at ?? null,
                        cell: (p) =>
                            p.resumed_at ? (
                                formatDateTime(p.resumed_at)
                            ) : (
                                <span className="text-muted-foreground">
                                    {t('campaigns.engine.stillPaused')}
                                </span>
                            ),
                    },
                    {
                        id: 'changes',
                        header: t('campaigns.engine.changes'),
                        cell: (p) => (
                            <span className="text-xs break-words">
                                {describeChanges(p.changes) || '—'}
                            </span>
                        ),
                    },
                ]}
            />
        </section>
    );
}
