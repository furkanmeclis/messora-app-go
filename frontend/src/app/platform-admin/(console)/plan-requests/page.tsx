import type { Metadata } from 'next';
import { Inbox } from 'lucide-react';
import { getT } from '@/i18n/server';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { Heading } from '../heading';
import { RequestsClient } from './requests-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('plans.admin.requests.title') };
}

const STATUSES = ['open', 'done', 'declined'] as const;

/** Plan change requests of every company. */
export default async function PlanRequestsPage({
    searchParams,
}: {
    searchParams: Promise<{ status?: string }>;
}) {
    const { status: raw } = await searchParams;
    const status = (STATUSES as readonly string[]).includes(raw ?? '')
        ? raw
        : raw === 'all'
          ? undefined
          : 'open';
    const [t, requests, plans] = await Promise.all([
        getT(),
        papi<Schemas['PlatformPlanRequest'][]>(
            `/platform/plan-requests${status ? `?status=${status}` : ''}`,
        ).then((r) => r.data),
        papi<Schemas['PlatformPlan'][]>('/platform/plans')
            .then((r) => r.data)
            .catch(() => [] as Schemas['PlatformPlan'][]),
    ]);
    return (
        <>
            <Heading
                icon={<Inbox />}
                title={t('plans.admin.requests.title')}
                description={t('plans.admin.requests.description')}
            />
            <RequestsClient
                requests={requests}
                plans={plans}
                status={status ?? 'all'}
            />
        </>
    );
}
