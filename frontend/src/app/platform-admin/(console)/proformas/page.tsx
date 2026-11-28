import type { Metadata } from 'next';
import { FileText } from 'lucide-react';
import { getT } from '@/i18n/server';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { FilterLinks } from '../billing-ui';
import { Heading } from '../heading';
import { ProformasTable } from './proformas-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('billing.admin.proformas.title') };
}

const STATUSES = ['open', 'partially_paid', 'paid', 'void'] as const;

/** Proformas of every company. */
export default async function ProformasPage({
    searchParams,
}: {
    searchParams: Promise<{ status?: string }>;
}) {
    const { status: raw } = await searchParams;
    const status = (STATUSES as readonly string[]).includes(raw ?? '')
        ? raw
        : undefined;
    const [t, rows] = await Promise.all([
        getT(),
        papi<Schemas['Proforma'][]>(
            `/platform/billing/proformas${status ? `?status=${status}` : ''}`,
        ).then((r) => r.data),
    ]);
    return (
        <>
            <Heading
                icon={<FileText />}
                title={t('billing.admin.proformas.title')}
                description={t('billing.admin.proformas.description')}
            />
            <div className="grid gap-4">
                <FilterLinks
                    base="/proformas"
                    current={status ?? 'all'}
                    options={[
                        ...STATUSES.map((s) => ({
                            value: s,
                            label: t(`billing.proforma.state.${s}`),
                        })),
                        {
                            value: 'all',
                            label: t('billing.admin.proformas.all'),
                        },
                    ]}
                />
                <ProformasTable rows={rows} />
            </div>
        </>
    );
}
