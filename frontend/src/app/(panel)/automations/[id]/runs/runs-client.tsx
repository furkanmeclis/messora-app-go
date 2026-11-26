'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { ClientDataTable } from '@/components/client-data-table';
import { useEffect, useState, useTransition } from 'react';
import { AlertCircle, History, Workflow } from 'lucide-react';
import { notify } from '@/components/notify';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
    Empty,
    EmptyDescription,
    EmptyHeader,
    EmptyMedia,
    EmptyTitle,
} from '@/components/ui/empty';
import {
    Sheet,
    SheetContent,
    SheetDescription,
    SheetHeader,
    SheetTitle,
} from '@/components/ui/sheet';
import { Skeleton } from '@/components/ui/skeleton';
import { Spinner } from '@/components/ui/spinner';
import {
    useFormatPrecise,
    useFormatDateTime,
} from '@/components/display-preferences';
import { useT } from '@/i18n/client';
import type { AutomationRun, AutomationRunStep } from '@/lib/types';
import { cn } from '@/lib/utils';
import { getRunAction, moreRunsAction } from '../../actions';
import { formatDuration, mk, stepLabel } from '../../model';

const STATUS_CLASS: Record<AutomationRun['status'], string> = {
    queued: 'bg-muted text-muted-foreground',
    running: 'bg-sky-500/15 text-sky-700 dark:text-sky-300',
    waiting: 'bg-amber-500/15 text-amber-700 dark:text-amber-300',
    completed: 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300',
    failed: 'bg-destructive/10 text-destructive',
    cancelled: 'bg-muted text-muted-foreground line-through',
};

const OUTCOME: Record<
    AutomationRunStep['outcome'],
    { className: string; dot: string }
> = {
    ok: {
        className: 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300',
        dot: 'bg-emerald-500',
    },
    skipped: {
        className: 'bg-muted text-muted-foreground',
        dot: 'bg-muted-foreground',
    },
    failed: {
        className: 'bg-destructive/10 text-destructive',
        dot: 'bg-destructive',
    },
    waiting: {
        className: 'bg-amber-500/15 text-amber-700 dark:text-amber-300',
        dot: 'bg-amber-500',
    },
    retrying: {
        className: 'bg-orange-500/15 text-orange-700 dark:text-orange-300',
        dot: 'bg-orange-500',
    },
};

const REASONS = [
    'no_conversation',
    'assignee_unavailable',
    'email_not_configured',
    'http_not_configured',
    'inbox_unavailable',
    'contact_ai_disabled',
    'ai_unavailable',
    'already_owner',
    'empty_note',
    'no_groups',
];

/** Failure codes of a run (automation_runs.error_code). */
export const ERROR_CODES = [
    'ai_disabled',
    'ai_not_configured',
    'ai_not_allowed',
    'ai_no_profile',
    'ai_error',
    'retries_exhausted',
    'step_failed',
];

export function StatusBadge({ status }: { status: AutomationRun['status'] }) {
    const t = useT();
    return (
        <Badge
            variant="outline"
            className={cn('border-0', STATUS_CLASS[status] ?? '')}
        >
            {STATUS_CLASS[status]
                ? t(mk(`automations.runs.status.${status}`))
                : status}
        </Badge>
    );
}

export function RunList({
    automationId,
    initial,
    initialCursor,
}: {
    automationId: string;
    initial: AutomationRun[];
    initialCursor?: string;
}) {
    const t = useT();
    const formatDateTime = useFormatDateTime();
    const router = useRouter();
    const [more, setMore] = useState<AutomationRun[]>([]);
    const [cursor, setCursor] = useState<string | undefined>(undefined);
    const [loaded, setLoaded] = useState(false);
    const [pending, startTransition] = useTransition();
    const [open, setOpen] = useState<string | null>(null);
    // A refresh replaces the first page; extra pages stay below it.
    const ids = new Set(initial.map((r) => r.id));
    const runs = [...initial, ...more.filter((r) => !ids.has(r.id))];
    const next = loaded ? cursor : initialCursor;

    if (runs.length === 0) {
        return (
            <Empty className="border py-12">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <History />
                    </EmptyMedia>
                    <EmptyTitle>{t('automations.runs.emptyTitle')}</EmptyTitle>
                    <EmptyDescription>
                        {t('automations.runs.emptyDescription')}
                    </EmptyDescription>
                </EmptyHeader>
            </Empty>
        );
    }

    const canvasHref = (r: AutomationRun) =>
        `/automations/${automationId}/runs/${r.id}`;

    return (
        <div className="space-y-3">
            <ClientDataTable
                rows={runs}
                rowId={(r) => r.id}
                storageKey="automation-runs"
                rowTestId="run-row"
                searchText={(r) =>
                    `${r.phone ?? ''} ${r.error ?? ''} ${t(
                        mk(`automations.runs.status.${r.status}`),
                    )}`
                }
                searchPlaceholder={t('table.search')}
                noMatch={
                    <p className="p-8 text-center text-sm text-muted-foreground">
                        {t('table.noMatch')}
                    </p>
                }
                columns={[
                    {
                        id: 'status',
                        header: t('automations.runs.columns.status'),
                        sortValue: (r) => r.status,
                        cell: (r) => (
                            <>
                                <StatusBadge status={r.status} />
                                {r.status === 'waiting' && r.resume_at && (
                                    <div className="mt-1 text-xs text-muted-foreground">
                                        {t('automations.runs.until', {
                                            time: formatDateTime(r.resume_at),
                                        })}
                                    </div>
                                )}
                            </>
                        ),
                    },
                    {
                        id: 'contact',
                        header: t('automations.runs.columns.contact'),
                        sortValue: (r) => r.phone ?? null,
                        cell: (r) => (
                            <span className="font-mono text-sm">
                                {r.contact_id && r.phone ? (
                                    <Link
                                        href={`/contacts/${r.contact_id}`}
                                        className="underline-offset-4 hover:underline"
                                    >
                                        {r.phone}
                                    </Link>
                                ) : (
                                    (r.phone ?? '—')
                                )}
                            </span>
                        ),
                    },
                    {
                        id: 'started',
                        header: t('automations.runs.columns.started'),
                        sortValue: (r) => r.created_at,
                        cell: (r) => (
                            <span className="text-sm whitespace-nowrap text-muted-foreground">
                                {formatDateTime(r.created_at)}
                            </span>
                        ),
                    },
                    {
                        id: 'finished',
                        header: t('automations.runs.columns.finished'),
                        sortValue: (r) => r.finished_at ?? null,
                        cell: (r) => (
                            <span className="text-sm whitespace-nowrap text-muted-foreground">
                                {formatDateTime(r.finished_at)}
                            </span>
                        ),
                    },
                    {
                        id: 'error',
                        header: t('automations.runs.columns.error'),
                        sortValue: (r) => r.error ?? null,
                        cell: (r) => (
                            <div
                                className="max-w-64 truncate text-sm text-destructive"
                                title={r.error ?? undefined}
                            >
                                {r.error ?? ''}
                            </div>
                        ),
                    },
                ]}
                rowActions={(r) => (
                    <div className="flex justify-end gap-1">
                        {(r.visited?.length ?? 0) > 0 && (
                            <Button
                                asChild
                                size="icon-sm"
                                variant="ghost"
                                aria-label={t(
                                    'automations.runs.viewOnCanvasOf',
                                    { time: formatDateTime(r.created_at) },
                                )}
                            >
                                <Link href={canvasHref(r)}>
                                    <Workflow />
                                </Link>
                            </Button>
                        )}
                        <Button
                            size="sm"
                            variant="outline"
                            onClick={() => setOpen(r.id)}
                            aria-label={t('automations.runs.detailsOf', {
                                time: formatDateTime(r.created_at),
                            })}
                        >
                            {t('automations.runs.details')}
                        </Button>
                    </div>
                )}
                rowMenu={(r) => [
                    [
                        {
                            id: 'details',
                            label: t('automations.runs.details'),
                            icon: History,
                            run: () => setOpen(r.id),
                        },
                        ...((r.visited?.length ?? 0) > 0
                            ? [
                                  {
                                      id: 'canvas',
                                      label: t('automations.runs.viewOnCanvas'),
                                      icon: Workflow,
                                      run: () => router.push(canvasHref(r)),
                                  },
                              ]
                            : []),
                    ],
                ]}
                onRowClick={(r) => setOpen(r.id)}
            />
            {next && (
                <div className="text-center">
                    <Button
                        variant="outline"
                        disabled={pending}
                        onClick={() =>
                            startTransition(async () => {
                                const r = await moreRunsAction(
                                    automationId,
                                    next,
                                );
                                if (notify(r)) {
                                    setMore((m) => [...m, ...r.data.items]);
                                    setCursor(r.data.next);
                                    setLoaded(true);
                                }
                            })
                        }
                    >
                        {pending && <Spinner />}{' '}
                        {t('automations.runs.loadMore')}
                    </Button>
                </div>
            )}
            <Sheet
                open={open !== null}
                onOpenChange={(o) => !o && setOpen(null)}
            >
                <SheetContent className="w-full overflow-y-auto sm:max-w-lg">
                    {open && (
                        <RunDetail
                            key={open}
                            runId={open}
                            automationId={automationId}
                        />
                    )}
                </SheetContent>
            </Sheet>
        </div>
    );
}

function RunDetail({
    runId,
    automationId,
}: {
    runId: string;
    automationId: string;
}) {
    const t = useT();
    const formatPrecise = useFormatPrecise();
    const [state, setState] = useState<
        | { kind: 'loading' }
        | { kind: 'error'; message: string }
        | { kind: 'ready'; run: AutomationRun }
    >({ kind: 'loading' });
    useEffect(() => {
        let live = true;
        getRunAction(runId).then(
            (r) =>
                live &&
                setState(
                    r.ok
                        ? { kind: 'ready', run: r.data }
                        : { kind: 'error', message: r.message },
                ),
            () =>
                live &&
                setState({
                    kind: 'error',
                    message: t('automations.runs.loadError'),
                }),
        );
        return () => {
            live = false;
        };
    }, [runId, t]);

    return (
        <>
            <SheetHeader>
                <SheetTitle>{t('automations.runs.detailTitle')}</SheetTitle>
                <SheetDescription>
                    {state.kind === 'ready'
                        ? t('automations.runs.startedAt', {
                              time: formatPrecise(state.run.created_at),
                          })
                        : t('automations.runs.detailDescription')}
                </SheetDescription>
            </SheetHeader>
            <div className="space-y-6 px-4 pb-6">
                {state.kind === 'loading' && (
                    <div className="space-y-3" aria-busy="true">
                        <span className="sr-only">
                            {t('automations.runs.loading')}
                        </span>
                        <Skeleton className="h-6 w-1/3" />
                        <Skeleton className="h-16 w-full" />
                        <Skeleton className="h-16 w-full" />
                    </div>
                )}
                {state.kind === 'error' && (
                    <Alert variant="destructive">
                        <AlertCircle />
                        <AlertDescription>{state.message}</AlertDescription>
                    </Alert>
                )}
                {state.kind === 'ready' && (
                    <RunBody run={state.run} automationId={automationId} />
                )}
            </div>
        </>
    );
}

/** Status, contact, times and error of a run. */
export function RunSummary({ run }: { run: AutomationRun }) {
    const t = useT();
    const formatPrecise = useFormatPrecise();
    return (
        <dl className="grid grid-cols-[8rem_1fr] gap-x-3 gap-y-2 text-sm">
            <dt className="text-muted-foreground">
                {t('automations.runs.columns.status')}
            </dt>
            <dd>
                <StatusBadge status={run.status} />
            </dd>
            {run.phone && (
                <>
                    <dt className="text-muted-foreground">
                        {t('automations.runs.columns.contact')}
                    </dt>
                    <dd className="font-mono">{run.phone}</dd>
                </>
            )}
            <dt className="text-muted-foreground">
                {t('automations.runs.columns.started')}
            </dt>
            <dd>{formatPrecise(run.created_at)}</dd>
            <dt className="text-muted-foreground">
                {t('automations.runs.columns.finished')}
            </dt>
            <dd>{formatPrecise(run.finished_at)}</dd>
            {run.resume_at && run.status === 'waiting' && (
                <>
                    <dt className="text-muted-foreground">
                        {t('automations.runs.resumes')}
                    </dt>
                    <dd>{formatPrecise(run.resume_at)}</dd>
                </>
            )}
            {run.error_code && ERROR_CODES.includes(run.error_code) && (
                <>
                    <dt className="text-muted-foreground">
                        {t('automations.runs.reason')}
                    </dt>
                    <dd
                        className="text-destructive"
                        data-testid="run-error-code"
                    >
                        {t(mk(`automations.runs.errorCodes.${run.error_code}`))}
                        {run.error_code.startsWith('ai_') &&
                            run.error_code !== 'ai_error' && (
                                <>
                                    {' '}
                                    <Link
                                        href="/settings/ai"
                                        className="font-medium underline underline-offset-2"
                                    >
                                        {t('automations.node.aiSettings')}
                                    </Link>
                                </>
                            )}
                    </dd>
                </>
            )}
            {run.error && (
                <>
                    <dt className="text-muted-foreground">
                        {t('automations.runs.columns.error')}
                    </dt>
                    <dd className="break-words text-destructive">
                        {run.error}
                    </dd>
                </>
            )}
        </dl>
    );
}

function RunBody({
    run,
    automationId,
}: {
    run: AutomationRun;
    automationId: string;
}) {
    const t = useT();
    const steps = run.steps ?? [];
    return (
        <>
            <RunSummary run={run} />
            {(run.visited?.length ?? 0) > 0 && (
                <Button asChild variant="outline" size="sm">
                    <Link href={`/automations/${automationId}/runs/${run.id}`}>
                        <Workflow /> {t('automations.runs.viewOnCanvas')}
                    </Link>
                </Button>
            )}

            <section aria-labelledby="run-event">
                <h3 id="run-event" className="mb-2 text-sm font-medium">
                    {t('automations.runs.event')}
                </h3>
                <Detail data={run.event} />
            </section>

            <section aria-labelledby="run-steps">
                <h3 id="run-steps" className="mb-2 text-sm font-medium">
                    {t('automations.runs.executedSteps')}
                </h3>
                {steps.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                        {t('automations.runs.noStepYet')}
                    </p>
                ) : (
                    <ol className="relative space-y-4 border-l pl-5">
                        {steps.map((s, i) => (
                            <TimelineItem key={`${s.pc}-${i}`} step={s} />
                        ))}
                    </ol>
                )}
            </section>
        </>
    );
}

/** One executed step of a run, with its outcome and details. */
export function TimelineItem({
    step,
    onSelect,
}: {
    step: AutomationRunStep;
    onSelect?: () => void;
}) {
    const t = useT();
    const formatPrecise = useFormatPrecise();
    const o = OUTCOME[step.outcome] ?? OUTCOME.ok;
    const isBranch = step.action === 'branch';
    const detail = { ...step.detail };
    let note: string | undefined;
    if (isBranch && typeof detail.result === 'boolean') {
        note = detail.result
            ? t('automations.runs.conditionTrue')
            : t('automations.runs.conditionFalse');
        delete detail.result;
    }
    if (step.action === 'delay' && typeof detail.seconds === 'number') {
        note = t(
            step.outcome === 'waiting'
                ? 'automations.runs.waiting'
                : 'automations.runs.waited',
            { duration: formatDuration(detail.seconds, t) },
        );
        delete detail.seconds;
    }
    if (
        step.outcome === 'retrying' &&
        typeof detail.attempt === 'number' &&
        typeof detail.max_retries === 'number'
    ) {
        note = t('automations.runs.retrying', {
            attempt: detail.attempt,
            max: detail.max_retries,
            duration: formatDuration(
                typeof detail.retry_in_seconds === 'number'
                    ? detail.retry_in_seconds
                    : 0,
                t,
            ),
        });
        delete detail.attempt;
        delete detail.max_retries;
        delete detail.retry_in_seconds;
    } else if (typeof detail.attempt === 'number' && detail.attempt > 1) {
        note = t('automations.runs.attempt', { attempt: detail.attempt });
        delete detail.attempt;
    }
    if (typeof detail.code === 'string') {
        const c = detail.code;
        if (ERROR_CODES.includes(c)) {
            note = [note, t(mk(`automations.runs.errorCodes.${c}`))]
                .filter(Boolean)
                .join(' ');
            delete detail.code;
        }
    }
    if (typeof detail.reason === 'string') {
        const r = detail.reason;
        note = REASONS.includes(r)
            ? t(mk(`automations.runs.reasons.${r}`))
            : r.startsWith('missing_')
              ? t('automations.runs.reasons.missing', { name: r.slice(8) })
              : undefined;
        if (note) delete detail.reason;
    }
    const label = stepLabel(step.action, t);
    return (
        <li data-testid="run-step" data-outcome={step.outcome}>
            <span
                className={cn(
                    'absolute -left-[5px] mt-1.5 size-2.5 rounded-full',
                    o.dot,
                )}
                aria-hidden
            />
            <div className="flex flex-wrap items-center gap-2">
                {onSelect ? (
                    <button
                        type="button"
                        onClick={onSelect}
                        className="text-sm font-medium underline-offset-4 hover:underline"
                    >
                        {label}
                    </button>
                ) : (
                    <span className="text-sm font-medium">{label}</span>
                )}
                <Badge
                    variant="outline"
                    className={cn('border-0', o.className)}
                >
                    {t(mk(`automations.runs.outcome.${step.outcome}`))}
                </Badge>
                <span className="ml-auto text-xs text-muted-foreground">
                    {formatPrecise(step.created_at)}
                </span>
            </div>
            {note && <p className="mt-1 text-sm">{note}</p>}
            {Object.keys(detail).length > 0 && (
                <div className="mt-2">
                    <Detail data={detail} />
                </div>
            )}
        </li>
    );
}

function humanize(key: string): string {
    const s = key.replace(/_/g, ' ');
    return s[0]!.toUpperCase() + s.slice(1);
}

/** A JSON object as a readable list of key/value pairs. */
function Detail({ data }: { data: Record<string, unknown> }) {
    const t = useT();
    const entries = Object.entries(data ?? {});
    if (entries.length === 0) {
        return (
            <p className="text-sm text-muted-foreground">
                {t('automations.runs.noDetails')}
            </p>
        );
    }
    return (
        <dl className="grid grid-cols-[8rem_1fr] gap-x-3 gap-y-1 rounded-md bg-muted/50 p-3 text-xs">
            {entries.map(([k, v]) => (
                <div key={k} className="contents">
                    <dt className="text-muted-foreground">{humanize(k)}</dt>
                    <dd className="min-w-0 font-mono break-words">
                        {v === null || v === undefined ? (
                            '—'
                        ) : typeof v === 'object' ? (
                            <pre className="whitespace-pre-wrap">
                                {JSON.stringify(v, null, 2)}
                            </pre>
                        ) : (
                            String(v)
                        )}
                    </dd>
                </div>
            ))}
        </dl>
    );
}
