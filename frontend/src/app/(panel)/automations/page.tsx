import type { Metadata } from 'next';
import Link from 'next/link';
import { Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Automation } from '@/lib/types';
import { PageHeading } from '../panel-header';
import { AutomationTable } from './automations-client';
import { loadNames } from './data';

export const metadata: Metadata = { title: 'Automations' };

export default async function AutomationsPage() {
    const me = await requirePermission('automations.read');
    const [list, names] = await Promise.all([
        api<Automation[]>('/automations'),
        loadNames(me),
    ]);
    const canManage = can(me, 'automations.manage');

    return (
        <>
            <PageHeading
                title="Automations"
                description="Reply, tag, branch and wait automatically when something happens."
                actions={
                    canManage && (
                        <Button asChild>
                            <Link href="/automations/new">
                                <Plus /> New automation
                            </Link>
                        </Button>
                    )
                }
            />
            <Card>
                <CardContent className="p-0">
                    <AutomationTable
                        automations={list.data}
                        phones={names.phones}
                        campaigns={names.campaigns}
                        canManage={canManage}
                    />
                </CardContent>
            </Card>
        </>
    );
}
