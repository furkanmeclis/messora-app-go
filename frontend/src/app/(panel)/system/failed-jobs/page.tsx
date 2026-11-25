import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { FailedJob } from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { RefreshButton } from '../../refresh-button';
import { FailedJobsTable } from './failed-jobs-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('failedJobs.title') };
}

/** The newest jobs are listed; older ones stay in the queue's archive. */
const LIMIT = 200;

export default async function FailedJobsPage() {
    const me = await requirePermission('settings.read');
    const [res, t] = await Promise.all([
        api<FailedJob[]>(`/system/failed-jobs?limit=${LIMIT}`),
        getT(),
    ]);
    const total = res.total ?? res.data.length;
    return (
        <>
            <PageHeading
                title={t('failedJobs.title')}
                description={t('failedJobs.description')}
                actions={<RefreshButton />}
            />
            {total > res.data.length && (
                <p className="mb-3 text-sm text-muted-foreground">
                    {t('failedJobs.showing', {
                        shown: res.data.length,
                        total,
                    })}
                </p>
            )}
            <FailedJobsTable
                jobs={res.data}
                canManage={can(me, 'settings.update')}
            />
        </>
    );
}
