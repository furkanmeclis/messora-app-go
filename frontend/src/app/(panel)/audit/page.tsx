import type { Metadata } from 'next';
import { Card, CardContent } from '@/components/ui/card';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { AuditLog } from '@/lib/types';
import { PageHeading } from '../panel-header';
import { AuditTable } from './audit-client';

export const metadata: Metadata = { title: 'Audit log' };

export default async function AuditPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    await requirePermission('audit.read');
    const action = (await searchParams).action;
    const q = new URLSearchParams({ limit: '50' });
    if (action) q.set('action', action);
    const first = await api<AuditLog[]>(`/audit-logs?${q}`);
    return (
        <>
            <PageHeading
                title="Audit log"
                description="Every security-relevant action, newest first. Entries cannot be changed or deleted."
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
