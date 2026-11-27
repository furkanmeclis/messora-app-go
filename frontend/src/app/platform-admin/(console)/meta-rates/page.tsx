import type { Metadata } from 'next';
import { Coins } from 'lucide-react';
import { getT } from '@/i18n/server';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { MetaRates } from '@/app/(panel)/settings/meta-rates/meta-rates-client';
import { Heading } from '../heading';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('platform.metaRates.title') };
}

/** The platform's single rate card (CSV import posts to /bff/meta-rates). */
export default async function PlatformMetaRatesPage() {
    const [t, rates] = await Promise.all([
        getT(),
        papi<Schemas['MetaRate'][]>('/platform/meta-rates').then((r) => r.data),
    ]);
    return (
        <div className="max-w-6xl">
            <Heading
                icon={<Coins />}
                title={t('platform.metaRates.title')}
                description={t('platform.metaRates.description')}
            />
            <MetaRates rates={rates} readOnly={false} />
        </div>
    );
}
