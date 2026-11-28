import type { Metadata } from 'next';
import { Cpu } from 'lucide-react';
import { getT } from '@/i18n/server';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { ModelsClient } from '@/app/(panel)/settings/ai/models/models-client';
import { Heading } from '../heading';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('platform.aiPrices.title') };
}

/** The platform's single AI price table. */
export default async function PlatformAIPricesPage() {
    const [t, prices] = await Promise.all([
        getT(),
        papi<Schemas['AIPrice'][]>('/platform/ai-prices').then((r) => r.data),
    ]);
    return (
        <>
            <Heading
                icon={<Cpu />}
                title={t('platform.aiPrices.title')}
                description={t('platform.aiPrices.description')}
            />
            <ModelsClient
                models={[]}
                prices={prices}
                fakeAvailable={false}
                canManage
                pricesOnly
            />
        </>
    );
}
