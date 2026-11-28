import type { Metadata } from 'next';
import Link from 'next/link';
import { notFound } from 'next/navigation';
import { ArrowLeft, Building2 } from 'lucide-react';
import { getT } from '@/i18n/server';
import { ApiError } from '@/lib/server/api';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { Heading } from '../../heading';
import { TenantDetail } from './tenant-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('platform.tenants.title') };
}

export default async function TenantPage({
    params,
}: {
    params: Promise<{ id: string }>;
}) {
    const { id } = await params;
    const [t, tenant] = await Promise.all([
        getT(),
        papi<Schemas['PlatformTenantDetail']>(`/platform/tenants/${id}`)
            .then((r) => r.data)
            .catch((err) => {
                if (
                    err instanceof ApiError &&
                    (err.status === 404 || err.status === 422)
                ) {
                    return null;
                }
                throw err;
            }),
    ]);
    if (!tenant) notFound();
    return (
        <>
            <Link
                href="/tenants"
                className="mb-4 inline-flex items-center gap-1.5 text-sm text-muted-foreground hover:text-foreground"
            >
                <ArrowLeft className="size-4" /> {t('platform.tenant.back')}
            </Link>
            <Heading
                icon={<Building2 />}
                title={tenant.name}
                description={tenant.primary_host ?? tenant.slug}
            />
            <TenantDetail tenant={tenant} />
        </>
    );
}
