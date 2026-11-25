'use client';

import { useMemo, useState, useTransition } from 'react';
import { CircleCheck, RotateCcw, Trash2 } from 'lucide-react';
import { toast } from 'sonner';
import { ClientDataTable } from '@/components/client-data-table';
import { useConfirm } from '@/components/confirm';
import { useFormatDateTime } from '@/components/display-preferences';
import { notify } from '@/components/notify';
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
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from '@/components/ui/select';
import { Spinner } from '@/components/ui/spinner';
import { useT } from '@/i18n/client';
import type { ActionResult, FailedJob } from '@/lib/types';
import { deleteFailedJobAction, retryFailedJobAction } from '../../actions';

const QUEUES = ['critical', 'campaigns', 'ai', 'default', 'bulk'] as const;

const key = (j: FailedJob) => `${j.queue}/${j.id}`;

export function FailedJobsTable({
    jobs,
    canManage,
}: {
    jobs: FailedJob[];
    canManage: boolean;
}) {
    const t = useT();
    const ask = useConfirm();
    const formatDateTime = useFormatDateTime();
    const [pending, startTransition] = useTransition();
    const [queue, setQueue] = useState<FailedJob['queue'] | 'all'>('all');
    const rows = useMemo(
        () => (queue === 'all' ? jobs : jobs.filter((j) => j.queue === queue)),
        [jobs, queue],
    );
    const byKey = useMemo(() => new Map(jobs.map((j) => [key(j), j])), [jobs]);

    const act = async (
        kind: 'retry' | 'delete',
        targets: FailedJob[],
        clear?: () => void,
    ) => {
        const one = targets.length === 1;
        const ok = await ask(
            kind === 'retry'
                ? {
                      title: one
                          ? t('failedJobs.retryTitle')
                          : t('failedJobs.retryManyTitle', {
                                count: targets.length,
                            }),
                      description: t('failedJobs.retryBody'),
                      confirmLabel: t('failedJobs.retry'),
                  }
                : {
                      title: one
                          ? t('failedJobs.deleteTitle')
                          : t('failedJobs.deleteManyTitle', {
                                count: targets.length,
                            }),
                      description: t('failedJobs.deleteBody'),
                      confirmLabel: t('failedJobs.delete'),
                      destructive: true,
                  },
        );
        if (!ok) return;
        const run =
            kind === 'retry' ? retryFailedJobAction : deleteFailedJobAction;
        const [first] = targets;
        startTransition(async () => {
            if (one && first) {
                const r = await run(first.queue, first.id);
                notify(r);
                if (r.ok) clear?.();
                return;
            }
            let done = 0;
            for (const j of targets) {
                const r: ActionResult = await run(j.queue, j.id);
                if (!r.ok) {
                    toast.error(r.message);
                    break;
                }
                done++;
            }
            if (done > 0) {
                toast.success(
                    t(
                        kind === 'retry'
                            ? 'failedJobs.retriedMany'
                            : 'failedJobs.deletedMany',
                        { count: done },
                    ),
                );
                clear?.();
            }
        });
    };

    const resolve = (ids: string[]) =>
        ids.map((id) => byKey.get(id)).filter((j): j is FailedJob => !!j);

    return (
        <ClientDataTable
            rows={rows}
            rowId={key}
            storageKey="failed-jobs"
            initialSort="-failedAt"
            rowTestId="failed-job-row"
            searchText={(j) =>
                `${j.type} ${j.queue} ${j.last_error} ${Object.values(j.refs).join(' ')}`
            }
            searchPlaceholder={t('table.search')}
            toolbar={
                <Select
                    value={queue}
                    onValueChange={(v) => setQueue(v as typeof queue)}
                >
                    <SelectTrigger
                        className="w-40"
                        aria-label={t('failedJobs.columns.queue')}
                    >
                        <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                        <SelectItem value="all">
                            {t('failedJobs.allQueues')}
                        </SelectItem>
                        {QUEUES.map((q) => (
                            <SelectItem key={q} value={q}>
                                {t(`failedJobs.queues.${q}`)}
                            </SelectItem>
                        ))}
                    </SelectContent>
                </Select>
            }
            empty={
                <Empty className="py-12">
                    <EmptyHeader>
                        <EmptyMedia variant="icon">
                            <CircleCheck />
                        </EmptyMedia>
                        <EmptyTitle>{t('failedJobs.empty')}</EmptyTitle>
                        <EmptyDescription>
                            {t('failedJobs.emptyHint')}
                        </EmptyDescription>
                    </EmptyHeader>
                </Empty>
            }
            noMatch={
                <p className="p-8 text-center text-sm text-muted-foreground">
                    {t('table.noMatch')}
                </p>
            }
            selectable={canManage}
            bulkActions={
                canManage
                    ? (ids, clear) => (
                          <>
                              <Button
                                  size="sm"
                                  variant="ghost"
                                  disabled={pending}
                                  onClick={() =>
                                      void act('retry', resolve(ids), clear)
                                  }
                              >
                                  {pending ? <Spinner /> : <RotateCcw />}
                                  {t('failedJobs.retry')}
                              </Button>
                              <Button
                                  size="sm"
                                  variant="ghost"
                                  disabled={pending}
                                  onClick={() =>
                                      void act('delete', resolve(ids), clear)
                                  }
                              >
                                  <Trash2 />
                                  {t('failedJobs.delete')}
                              </Button>
                          </>
                      )
                    : undefined
            }
            columns={[
                {
                    id: 'type',
                    header: t('failedJobs.columns.type'),
                    hideable: false,
                    sortValue: (j) => j.type,
                    cell: (j) => (
                        <div className="min-w-0">
                            <div className="font-mono text-sm font-medium">
                                {j.type}
                            </div>
                            <div className="font-mono text-xs break-all text-muted-foreground">
                                {j.id}
                            </div>
                        </div>
                    ),
                },
                {
                    id: 'queue',
                    header: t('failedJobs.columns.queue'),
                    sortValue: (j) => j.queue,
                    cell: (j) => (
                        <Badge variant="outline">
                            {t(`failedJobs.queues.${j.queue}`)}
                        </Badge>
                    ),
                },
                {
                    id: 'error',
                    header: t('failedJobs.columns.error'),
                    cell: (j) => (
                        <p
                            className="line-clamp-3 max-w-md text-sm break-words text-destructive"
                            title={j.last_error}
                        >
                            {j.last_error || '—'}
                        </p>
                    ),
                },
                {
                    id: 'refs',
                    header: t('failedJobs.columns.refs'),
                    defaultHidden: true,
                    cell: (j) => (
                        <dl className="space-y-0.5 font-mono text-xs text-muted-foreground">
                            {Object.entries(j.refs).map(([k, v]) => (
                                <div key={k} className="flex gap-1">
                                    <dt>{k}:</dt>
                                    <dd className="break-all">{v}</dd>
                                </div>
                            ))}
                        </dl>
                    ),
                },
                {
                    id: 'attempts',
                    header: t('failedJobs.columns.attempts'),
                    sortValue: (j) => j.retried,
                    cell: (j) => (
                        <span className="text-sm whitespace-nowrap text-muted-foreground">
                            {t('failedJobs.attempts', {
                                retried: j.retried,
                                max: j.max_retry,
                            })}
                        </span>
                    ),
                },
                {
                    id: 'failedAt',
                    header: t('failedJobs.columns.failedAt'),
                    sortValue: (j) => j.failed_at,
                    cell: (j) => (
                        <span className="text-sm whitespace-nowrap text-muted-foreground">
                            {formatDateTime(j.failed_at)}
                        </span>
                    ),
                },
            ]}
            rowActions={
                canManage
                    ? (j) => (
                          <div className="flex justify-end gap-2">
                              <Button
                                  size="sm"
                                  variant="outline"
                                  disabled={pending}
                                  onClick={() => void act('retry', [j])}
                              >
                                  <RotateCcw />
                                  {t('failedJobs.retry')}
                              </Button>
                              <Button
                                  size="sm"
                                  variant="ghost"
                                  disabled={pending}
                                  aria-label={t('failedJobs.delete')}
                                  onClick={() => void act('delete', [j])}
                              >
                                  <Trash2 />
                              </Button>
                          </div>
                      )
                    : undefined
            }
            rowMenu={
                canManage
                    ? (j) => [
                          [
                              {
                                  id: 'retry',
                                  label: t('failedJobs.retry'),
                                  icon: RotateCcw,
                                  disabled: pending,
                                  run: () => void act('retry', [j]),
                              },
                              {
                                  id: 'delete',
                                  label: t('failedJobs.delete'),
                                  icon: Trash2,
                                  danger: true,
                                  disabled: pending,
                                  run: () => void act('delete', [j]),
                              },
                          ],
                      ]
                    : undefined
            }
        />
    );
}
