import type { Metadata } from 'next';
import { Landmark } from 'lucide-react';
import { getT } from '@/i18n/server';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { Heading } from '../heading';
import { PaymentsClient } from './payments-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('billing.admin.payments.title') };
}

const STATUSES = ['unmatched', 'matched', 'ignored'] as const;

/** The payments inbox: incoming bank transfers and their matches. */
export default async function PaymentsPage({
    searchParams,
}: {
    searchParams: Promise<{ status?: string }>;
}) {
    const { status: raw } = await searchParams;
    const status = (STATUSES as readonly string[]).includes(raw ?? '')
        ? raw
        : raw === 'all'
          ? undefined
          : 'unmatched';
    const [t, transfers] = await Promise.all([
        getT(),
        papi<Schemas['BankTransfer'][]>(
            `/platform/billing/transfers${status ? `?status=${status}` : ''}`,
        ).then((r) => r.data),
    ]);
    return (
        <>
            <Heading
                icon={<Landmark />}
                title={t('billing.admin.payments.title')}
                description={t('billing.admin.payments.description')}
            />
            <PaymentsClient transfers={transfers} status={status ?? 'all'} />
        </>
    );
}
