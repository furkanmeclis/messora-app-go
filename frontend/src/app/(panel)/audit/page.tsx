import type { Metadata } from 'next';
import { Card, CardContent } from '@/components/ui/card';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { AuditLog } from '@/lib/types';
import { PageHeading } from '../panel-header';
import { AuditTable } from './audit-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('audit.title') };
}

export default async function AuditPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    await requirePermission('audit.read');
    const action = (await searchParams).action;
    const q = new URLSearchParams({ limit: '50' });
    if (action) q.set('action', action);
    const [first, t] = await Promise.all([
        api<AuditLog[]>(`/audit-logs?${q}`),
        getT(),
    ]);
    return (
        <>
            <PageHeading
                title={t('audit.title')}
                description={t('audit.description')}
            />
            <Card>
                <CardContent className="p-0">
                    <AuditTable
                        key={action ?? ''}
                        initial={first.data}
                        initialCursor={first.nextCursor}
                        action={action}
                    />
                </CardContent>
            </Card>
        </>
    );
}
