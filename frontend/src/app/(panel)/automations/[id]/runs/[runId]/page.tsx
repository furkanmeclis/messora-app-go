import type { Metadata } from 'next';
import Link from 'next/link';
import { notFound } from 'next/navigation';
import { ArrowLeft } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { getT } from '@/i18n/server';
import { api, ApiError } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { AutomationRun } from '@/lib/types';
import { PageHeading } from '../../../../panel-header';
import { RefreshButton } from '../../../../refresh-button';
import { loadAutomation, loadNames } from '../../../data';
import { mk } from '../../../model';
import { RunTrace } from '../../../trace';

const UUID = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/;

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('automations.trace.metaTitle') };
}

export default async function RunCanvasPage({
    params,
}: {
    params: Promise<{ id: string; runId: string }>;
}) {
    const me = await requirePermission('automations.read');
    const t = await getT();
    const { id, runId } = await params;
    if (!UUID.test(runId)) notFound();
    const [automation, names] = await Promise.all([
        loadAutomation(id),
        loadNames(me),
    ]);
    let run: AutomationRun;
    try {
        run = (await api<AutomationRun>(`/automation-runs/${runId}`)).data;
    } catch (err) {
        if (err instanceof ApiError && err.status === 404) notFound();
        throw err;
    }
    if (run.automation_id !== automation.id) notFound();
    return (
        <>
            <PageHeading
                title={t('automations.trace.title', { name: automation.name })}
                description={t('automations.trace.description', {
                    status: t(mk(`automations.runs.status.${run.status}`)),
                })}
                actions={
                    <div className="flex gap-2">
                        <Button asChild variant="outline" size="sm">
                            <Link href={`/automations/${id}/runs`}>
                                <ArrowLeft />{' '}
                                {t('automations.trace.backToRuns')}
                            </Link>
                        </Button>
                        <RefreshButton />
                    </div>
                }
            />
            <RunTrace automation={automation} run={run} names={names} />
        </>
    );
}
