import type { Metadata } from 'next';
import { Settings } from 'lucide-react';
import { getT } from '@/i18n/server';
import { papi } from '@/lib/server/platform';
import type { Schemas } from '@/lib/types';
import { Heading } from '../heading';
import { SettingsForm } from './settings-form';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('platform.settings.title') };
}

export default async function PlatformSettingsPage() {
    const [t, settings] = await Promise.all([
        getT(),
        papi<Schemas['PlatformSettings']>('/platform/settings').then(
            (r) => r.data,
        ),
    ]);
    return (
        <>
            <Heading
                icon={<Settings />}
                title={t('platform.settings.title')}
                description={t('platform.settings.description')}
            />
            <SettingsForm settings={settings} />
        </>
    );
}
