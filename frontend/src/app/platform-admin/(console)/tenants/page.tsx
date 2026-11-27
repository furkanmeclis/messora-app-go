import type { Metadata } from 'next';
import { Building2 } from 'lucide-react';
import { getT } from '@/i18n/server';
import { configuredBaseDomain } from '@/lib/platform-host';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { Heading } from '../heading';
import { CreateTenant, TenantList } from './tenants-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('platform.tenants.title') };
}

const STATUSES = ['active', 'suspended', 'deleted'] as const;

export default async function TenantsPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const sp = await searchParams;
    const q = new URLSearchParams({ limit: '50' });
    if (sp.q) q.set('q', sp.q);
    const status = STATUSES.find((s) => s === sp.status);
    if (status) q.set('status', status);
    const [t, first] = await Promise.all([
        getT(),
        papi<Schemas['PlatformTenant'][]>(`/platform/tenants?${q}`),
    ]);
    return (
        <>
            <Heading
                icon={<Building2 />}
                title={t('platform.tenants.title')}
                description={t('platform.tenants.description')}
                actions={<CreateTenant baseDomain={configuredBaseDomain()} />}
            />
            <TenantList
                key={q.toString()}
                initial={first.data}
                initialCursor={first.nextCursor}
                query={sp.q ?? ''}
                status={status ?? ''}
            />
        </>
    );
}
