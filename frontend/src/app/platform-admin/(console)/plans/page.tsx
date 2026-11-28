import type { Metadata } from 'next';
import { Gem } from 'lucide-react';
import { getT } from '@/i18n/server';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { Heading } from '../heading';
import { PlansClient } from './plans-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('plans.admin.plans.title') };
}

/** The plan catalogue: limits, features, list prices and visibility. */
export default async function PlatformPlansPage() {
    const [t, plans] = await Promise.all([
        getT(),
        papi<Schemas['PlatformPlan'][]>('/platform/plans').then((r) => r.data),
    ]);
    return (
        <>
            <Heading
                icon={<Gem />}
                title={t('plans.admin.plans.title')}
                description={t('plans.admin.plans.description')}
            />
            <PlansClient plans={plans} />
        </>
    );
}
