import type { Metadata } from 'next';
import Link from 'next/link';
import { ArrowLeft } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { AutomationRun } from '@/lib/types';
import { PageHeading } from '../../../panel-header';
import { RefreshButton } from '../../../refresh-button';
import { loadAutomation } from '../../data';
import { describeTrigger } from '../../model';
import { RunList } from './runs-client';

export const metadata: Metadata = { title: 'Automation runs' };

export default async function AutomationRunsPage({
    params,
}: {
    params: Promise<{ id: string }>;
}) {
    await requirePermission('automations.read');
    const { id } = await params;
    const automation = await loadAutomation(id);
    const runs = await api<AutomationRun[]>(`/automations/${id}/runs?limit=50`);
    return (
        <>
            <PageHeading
                title={`Runs · ${automation.name}`}
                description={describeTrigger(
                    automation.trigger_type,
                    automation.trigger,
                )}
                actions={
                    <div className="flex gap-2">
                        <Button asChild variant="outline" size="sm">
                            <Link href={`/automations/${id}`}>
                                <ArrowLeft /> Automation
                            </Link>
                        </Button>
                        <RefreshButton />
                    </div>
                }
            />
            <Card>
                <CardContent className="p-0">
                    <RunList
                        automationId={id}
                        initial={runs.data}
                        initialCursor={runs.nextCursor}
                    />
                </CardContent>
            </Card>
        </>
    );
}
