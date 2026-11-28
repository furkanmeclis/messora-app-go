import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Schemas } from '@/lib/types';
import { PlanPage } from './plan-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('plans.title') };
}

export default async function PlanSettingsPage() {
    const me = await requirePermission('settings.read');
    const [overview, plans] = await Promise.all([
        api<Schemas['PlanOverview']>('/plan').then((r) => r.data),
        api<Schemas['CatalogPlan'][]>('/plans').then((r) => r.data),
    ]);
    return (
        <div className="max-w-6xl">
            <PlanPage
                overview={overview}
                plans={plans}
                canRequest={can(me, 'settings.update') && !me.support}
            />
        </div>
    );
}
