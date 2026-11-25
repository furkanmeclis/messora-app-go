import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Schemas } from '@/lib/types';
import { MetaRates } from './meta-rates-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('settings.tabs.metaRates') };
}

export default async function MetaRatesPage() {
    const me = await requirePermission('settings.read');
    const rates = await api<Schemas['MetaRate'][]>('/settings/meta-rates');
    return (
        <div className="max-w-6xl">
            <MetaRates
                rates={rates.data}
                readOnly={!can(me, 'settings.update')}
            />
        </div>
    );
}
