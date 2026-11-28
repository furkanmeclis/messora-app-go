import type { Metadata } from 'next';
import { Sparkles } from 'lucide-react';
import { getT } from '@/i18n/server';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { Heading } from '../heading';
import { TechProviderForm } from './tech-provider-form';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('platform.techProvider.title') };
}

/** Messora as Meta Tech Provider (Embedded Signup): the platform switch and set-up. */
export default async function TechProviderPage() {
    const [t, config] = await Promise.all([
        getT(),
        papi<Schemas['PlatformTechProvider']>(
            '/platform/meta/tech-provider',
        ).then((r) => r.data),
    ]);
    return (
        <>
            <Heading
                icon={<Sparkles />}
                title={t('platform.techProvider.title')}
                description={t('platform.techProvider.description')}
            />
            <TechProviderForm config={config} />
        </>
    );
}
