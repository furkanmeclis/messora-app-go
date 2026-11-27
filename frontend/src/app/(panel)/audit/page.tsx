import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { AuditLog, Schemas } from '@/lib/types';
import { PageHeading } from '../panel-header';
import { AuditTable } from './audit-client';
import { SupportAccess } from './support-access';

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
    const [first, t, grants] = await Promise.all([
        api<AuditLog[]>(`/audit-logs?${q}`),
        getT(),
        api<Schemas['SupportGrant'][]>('/support-access')
            .then((r) => r.data)
            .catch(() => [] as Schemas['SupportGrant'][]),
    ]);
    return (
        <>
            <PageHeading
                title={t('audit.title')}
                description={t('audit.description')}
            />
            {!action && <SupportAccess grants={grants} />}
            <AuditTable
                key={action ?? ''}
                initial={first.data}
                initialCursor={first.nextCursor}
                action={action}
            />
        </>
    );
}
