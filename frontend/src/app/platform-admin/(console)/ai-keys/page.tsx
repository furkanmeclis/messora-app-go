import type { Metadata } from 'next';
import { KeyRound } from 'lucide-react';
import { getT } from '@/i18n/server';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { Heading } from '../heading';
import { AIKeysClient, AISettingsCard } from './ai-keys-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('plans.admin.aiKeys.title') };
}

/** Messora's own AI provider keys and the AI credit conversion. */
export default async function PlatformAIKeysPage() {
    const [t, providers, settings] = await Promise.all([
        getT(),
        papi<Schemas['PlatformAIProvider'][]>('/platform/ai-providers').then(
            (r) => r.data,
        ),
        papi<Schemas['PlatformAISettings']>('/platform/ai-settings').then(
            (r) => r.data,
        ),
    ]);
    return (
        <>
            <Heading
                icon={<KeyRound />}
                title={t('plans.admin.aiKeys.title')}
                description={t('plans.admin.aiKeys.description')}
            />
            <div className="grid max-w-5xl gap-6">
                <AIKeysClient providers={providers} />
                <AISettingsCard settings={settings} providers={providers} />
            </div>
        </>
    );
}
