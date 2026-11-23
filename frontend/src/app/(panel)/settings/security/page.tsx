import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Settings } from '@/lib/types';
import { SecurityForm } from './security-form';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('settings.tabs.security') };
}

export default async function SecuritySettingsPage() {
    const me = await requirePermission('settings.read');
    const settings = (await api<Settings>('/settings')).data;
    return (
        <SecurityForm
            settings={settings}
            readOnly={!can(me, 'settings.update')}
        />
    );
}
