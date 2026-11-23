import type { Metadata } from 'next';
import Link from 'next/link';
import { Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Automation } from '@/lib/types';
import { PageHeading } from '../panel-header';
import { AutomationTable } from './automations-client';
import { loadNames } from './data';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('automations.list.title') };
}

export default async function AutomationsPage() {
    const me = await requirePermission('automations.read');
    const t = await getT();
    const [list, names] = await Promise.all([
        api<Automation[]>('/automations'),
        loadNames(me),
    ]);
    const canManage = can(me, 'automations.manage');

    return (
        <>
            <PageHeading
                title={t('automations.list.title')}
                description={t('automations.list.description')}
                actions={
                    canManage && (
                        <Button asChild>
                            <Link href="/automations/new">
                                <Plus /> {t('automations.new.title')}
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
