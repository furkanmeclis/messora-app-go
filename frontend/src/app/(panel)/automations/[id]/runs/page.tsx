import type { Metadata } from 'next';
import Link from 'next/link';
import { ArrowLeft } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { AutomationRun } from '@/lib/types';
import { PageHeading } from '../../../panel-header';
import { RefreshButton } from '../../../refresh-button';
import { loadAutomation, loadNames } from '../../data';
import { describeTrigger } from '../../model';
import { RunList } from './runs-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('automations.runs.metaTitle') };
}

export default async function AutomationRunsPage({
    params,
}: {
    params: Promise<{ id: string }>;
}) {
    const me = await requirePermission('automations.read');
    const t = await getT();
    const { id } = await params;
    const [automation, names] = await Promise.all([
        loadAutomation(id),
        loadNames(me),
    ]);
    const runs = await api<AutomationRun[]>(`/automations/${id}/runs?limit=50`);
    return (
        <>
            <PageHeading
                title={t('automations.runs.title', { name: automation.name })}
                description={describeTrigger(
                    automation.trigger_type,
                    automation.trigger,
                    t,
                    {
                        phones: new Map(
                            names.phones.map((p) => [p.id, p.label]),
                        ),
                        campaigns: new Map(
                            names.campaigns.map((c) => [c.id, c.label]),
                        ),
                    },
                )}
                actions={
                    <div className="flex gap-2">
                        <Button asChild variant="outline" size="sm">
                            <Link href={`/automations/${id}`}>
                                <ArrowLeft /> {t('automations.runs.back')}
                            </Link>
                        </Button>
                        <RefreshButton />
                    </div>
                }
            />
            <RunList
                automationId={id}
                initial={runs.data}
                initialCursor={runs.nextCursor}
            />
        </>
    );
}
