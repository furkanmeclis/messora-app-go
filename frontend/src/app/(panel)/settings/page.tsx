import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Settings } from '@/lib/types';
import { GeneralForm } from './general-form';
import { LogoUploader } from './logo-uploader';
import { Section } from './section';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('settings.title') };
}

export default async function SettingsPage() {
    const me = await requirePermission('settings.read');
    const [settings, t] = await Promise.all([
        api<Settings>('/settings').then((r) => r.data),
        getT(),
    ]);
    const readOnly = !can(me, 'settings.update');
    return (
        <div className="space-y-6">
            <GeneralForm settings={settings} readOnly={readOnly} />
            <Section
                title={t('settings.logo.title')}
                description={t('settings.logo.hint')}
            >
                <LogoUploader logo={settings.logo} readOnly={readOnly} />
            </Section>
        </div>
    );
}
