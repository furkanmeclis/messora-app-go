'use client';

import Link from 'next/link';
import { useEffect, useState, useTransition } from 'react';
import { AlertCircle, History } from 'lucide-react';
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
    Table,
    TableBody,
    TableCell,
    TableHead,
    TableHeader,
    TableRow,
} from '@/components/ui/table';
import { formatDateTime } from '@/lib/format';
import type { AutomationRun, AutomationRunStep } from '@/lib/types';
import { cn } from '@/lib/utils';
import { getRunAction, moreRunsAction } from '../../actions';
import { formatDuration, stepLabel } from '../../model';

const STATUS: Record<
    AutomationRun['status'],
    { label: string; className: string }
> = {
    queued: { label: 'Queued', className: 'bg-muted text-muted-foreground' },
    running: {
        label: 'Running',
        className: 'bg-sky-500/15 text-sky-700 dark:text-sky-300',
    },
    waiting: {
        label: 'Waiting',
        className: 'bg-amber-500/15 text-amber-700 dark:text-amber-300',
    },
    completed: {
        label: 'Completed',
        className: 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300',
    },
    failed: {
        label: 'Failed',
        className: 'bg-destructive/10 text-destructive',
    },
    cancelled: {
        label: 'Cancelled',
        className: 'bg-muted text-muted-foreground line-through',
    },
};

const OUTCOME: Record<
    AutomationRunStep['outcome'],
    { label: string; className: string; dot: string }
> = {
    ok: {
        label: 'OK',
        className: 'bg-emerald-500/15 text-emerald-700 dark:text-emerald-300',
        dot: 'bg-emerald-500',
    },
    skipped: {
        label: 'Skipped',
        className: 'bg-muted text-muted-foreground',
        dot: 'bg-muted-foreground',
    },
    failed: {
        label: 'Failed',
        className: 'bg-destructive/10 text-destructive',
        dot: 'bg-destructive',
    },
    waiting: {
        label: 'Waiting',
        className: 'bg-amber-500/15 text-amber-700 dark:text-amber-300',
        dot: 'bg-amber-500',
    },
};

const REASONS: Record<string, string> = {
    no_conversation: 'The contact has no conversation.',
    assignee_unavailable: 'The person is not active any more.',
    email_not_configured: 'E-mail (SMTP) is not configured.',
    http_not_configured: 'HTTP requests are not configured.',
};

function StatusBadge({ status }: { status: AutomationRun['status'] }) {
    const s = STATUS[status] ?? { label: status, className: '' };
    return (
        <Badge variant="outline" className={cn('border-0', s.className)}>
            {s.label}
        </Badge>
    );
}

/** Date and time with seconds (step timelines are often seconds apart). */
function formatPrecise(iso: string | null | undefined): string {
    if (!iso) return '—';
    return new Intl.DateTimeFormat(undefined, {
        dateStyle: 'medium',
        timeStyle: 'medium',
    }).format(new Date(iso));
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
            <Empty className="py-12">
                <EmptyHeader>
                    <EmptyMedia variant="icon">
                        <History />
                    </EmptyMedia>
                    <EmptyTitle>No runs yet</EmptyTitle>
                    <EmptyDescription>
                        Every time the trigger fires for a contact, the run and
                        each executed step appear here.
                    </EmptyDescription>
                </EmptyHeader>
            </Empty>
        );
    }

    return (
        <div>
            <Table>
                <TableHeader>
                    <TableRow>
                        <TableHead>Status</TableHead>
                        <TableHead>Contact</TableHead>
                        <TableHead>Started</TableHead>
                        <TableHead>Finished</TableHead>
                        <TableHead>Error</TableHead>
                        <TableHead>
                            <span className="sr-only">Details</span>
                        </TableHead>
                    </TableRow>
                </TableHeader>
                <TableBody>
                    {runs.map((r) => (
                        <TableRow key={r.id} data-testid="run-row">
                            <TableCell>
                                <StatusBadge status={r.status} />
                                {r.status === 'waiting' && r.resume_at && (
                                    <div className="mt-1 text-xs text-muted-foreground">
                                        until {formatDateTime(r.resume_at)}
                                    </div>
                                )}
                            </TableCell>
                            <TableCell className="font-mono text-sm">
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
                            </TableCell>
                            <TableCell className="text-sm text-muted-foreground">
                                {formatDateTime(r.created_at)}
                            </TableCell>
                            <TableCell className="text-sm text-muted-foreground">
                                {formatDateTime(r.finished_at)}
                            </TableCell>
                            <TableCell className="max-w-64 truncate text-sm text-destructive">
                                <span title={r.error ?? undefined}>
                                    {r.error ?? ''}
                                </span>
                            </TableCell>
                            <TableCell className="text-right">
                                <Button
                                    size="sm"
                                    variant="outline"
                                    onClick={() => setOpen(r.id)}
                                    aria-label={`Details of the run started ${formatDateTime(r.created_at)}`}
                                >
                                    Details
                                </Button>
                            </TableCell>
                        </TableRow>
                    ))}
                </TableBody>
            </Table>
            {next && (
                <div className="border-t p-3 text-center">
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
                        {pending && <Spinner />} Load more
                    </Button>
                </div>
            )}
            <Sheet
                open={open !== null}
                onOpenChange={(o) => !o && setOpen(null)}
            >
                <SheetContent className="w-full overflow-y-auto sm:max-w-lg">
                    {open && <RunDetail key={open} runId={open} />}
                </SheetContent>
            </Sheet>
        </div>
    );
}

function RunDetail({ runId }: { runId: string }) {
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
                    message: 'The run could not be loaded.',
                }),
        );
        return () => {
            live = false;
        };
    }, [runId]);

    return (
        <>
            <SheetHeader>
                <SheetTitle>Run details</SheetTitle>
                <SheetDescription>
                    {state.kind === 'ready'
                        ? `Started ${formatPrecise(state.run.created_at)}`
                        : 'Executed steps and their outcome.'}
                </SheetDescription>
            </SheetHeader>
            <div className="space-y-6 px-4 pb-6">
                {state.kind === 'loading' && (
                    <div className="space-y-3" aria-busy="true">
                        <span className="sr-only">Loading run…</span>
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
                {state.kind === 'ready' && <RunBody run={state.run} />}
            </div>
        </>
    );
}

function RunBody({ run }: { run: AutomationRun }) {
    const steps = run.steps ?? [];
    return (
        <>
            <dl className="grid grid-cols-[8rem_1fr] gap-x-3 gap-y-2 text-sm">
                <dt className="text-muted-foreground">Status</dt>
                <dd>
                    <StatusBadge status={run.status} />
                </dd>
                {run.phone && (
                    <>
                        <dt className="text-muted-foreground">Contact</dt>
                        <dd className="font-mono">{run.phone}</dd>
                    </>
                )}
                <dt className="text-muted-foreground">Finished</dt>
                <dd>{formatPrecise(run.finished_at)}</dd>
                {run.resume_at && run.status === 'waiting' && (
                    <>
                        <dt className="text-muted-foreground">Resumes</dt>
                        <dd>{formatPrecise(run.resume_at)}</dd>
                    </>
                )}
                {run.error && (
                    <>
                        <dt className="text-muted-foreground">Error</dt>
                        <dd className="break-words text-destructive">
                            {run.error}
                        </dd>
                    </>
                )}
            </dl>

            <section aria-labelledby="run-event">
                <h3 id="run-event" className="mb-2 text-sm font-medium">
                    Triggering event
                </h3>
                <Detail data={run.event} />
            </section>

            <section aria-labelledby="run-steps">
                <h3 id="run-steps" className="mb-2 text-sm font-medium">
                    Executed steps
                </h3>
                {steps.length === 0 ? (
                    <p className="text-sm text-muted-foreground">
                        No step has run yet.
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

function TimelineItem({ step }: { step: AutomationRunStep }) {
    const o = OUTCOME[step.outcome] ?? OUTCOME.ok;
    const isBranch = step.action === 'branch';
    const detail = { ...step.detail };
    let note: string | undefined;
    if (isBranch && typeof detail.result === 'boolean') {
        note = detail.result
            ? 'Condition was true: continued with the then steps.'
            : 'Condition was false: continued with the else steps.';
        delete detail.result;
    }
    if (step.action === 'delay' && typeof detail.seconds === 'number') {
        note = `Waited ${formatDuration(detail.seconds)}.`;
        delete detail.seconds;
    }
    if (typeof detail.reason === 'string') {
        const r = detail.reason;
        note =
            REASONS[r] ??
            (r.startsWith('missing_')
                ? `A template variable resolved to nothing (${r.slice(8)}).`
                : undefined);
        if (note) delete detail.reason;
    }
    return (
        <li data-testid="run-step">
            <span
                className={cn(
                    'absolute -left-[5px] mt-1.5 size-2.5 rounded-full',
                    o.dot,
                )}
                aria-hidden
            />
            <div className="flex flex-wrap items-center gap-2">
                <span className="text-sm font-medium">
                    {isBranch ? 'Condition' : stepLabel(step.action)}
                </span>
                <Badge
                    variant="outline"
                    className={cn('border-0', o.className)}
                >
                    {o.label}
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
    const entries = Object.entries(data ?? {});
    if (entries.length === 0) {
        return <p className="text-sm text-muted-foreground">No details.</p>;
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
