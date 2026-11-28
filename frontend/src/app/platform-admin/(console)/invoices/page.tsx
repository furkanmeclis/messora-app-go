import type { Metadata } from 'next';
import { ReceiptText } from 'lucide-react';
import { getT } from '@/i18n/server';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { FilterLinks } from '../billing-ui';
import { Heading } from '../heading';
import { InvoicesTable } from './invoices-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('billing.admin.invoices.title') };
}

const STATUSES = ['pending', 'issued'] as const;

/** Invoice records of every company: record the official invoice. */
export default async function InvoicesPage({
    searchParams,
}: {
    searchParams: Promise<{ status?: string }>;
}) {
    const { status: raw } = await searchParams;
    const status = (STATUSES as readonly string[]).includes(raw ?? '')
        ? raw
        : raw === 'all'
          ? undefined
          : 'pending';
    const [t, rows] = await Promise.all([
        getT(),
        papi<Schemas['Invoice'][]>(
            `/platform/billing/invoices${status ? `?status=${status}` : ''}`,
        ).then((r) => r.data),
    ]);
    return (
        <>
            <Heading
                icon={<ReceiptText />}
                title={t('billing.admin.invoices.title')}
                description={t('billing.admin.invoices.description')}
            />
            <div className="grid gap-4">
                <FilterLinks
                    base="/invoices"
                    current={status ?? 'all'}
                    options={[
                        ...STATUSES.map((s) => ({
                            value: s,
                            label: t(`billing.docs.invoiceState.${s}`),
                        })),
                        {
                            value: 'all',
                            label: t('billing.admin.invoices.all'),
                        },
                    ]}
                />
                <InvoicesTable rows={rows} />
            </div>
        </>
    );
}
