import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Settings } from '@/lib/types';
import { Section } from '../section';
import { LogoUploader } from './logo-uploader';
import { ThemeEditor } from './theme-editor';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('appearance.title') };
}

export default async function AppearancePage() {
    const me = await requirePermission('settings.read');
    const [settings, t] = await Promise.all([
        api<Settings>('/settings').then((r) => r.data),
        getT(),
    ]);
    const isOwner = me.user.role.key === 'owner';
    return (
        <div className="space-y-6">
            <Section
                title={t('appearance.logo')}
                description={t('appearance.logoHint')}
                className="max-w-3xl"
            >
                <LogoUploader
                    logo={settings.logo}
                    readOnly={!can(me, 'settings.update')}
                />
            </Section>
            <ThemeEditor theme={settings.theme} readOnly={!isOwner} />
        </div>
    );
}
