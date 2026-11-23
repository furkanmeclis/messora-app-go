import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Settings } from '@/lib/types';
import { PwaForm } from './pwa-form';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('pwa.title') };
}

export default async function PwaSettingsPage() {
    const me = await requirePermission('settings.read');
    const settings = (await api<Settings>('/settings')).data;
    return (
        <PwaForm settings={settings} readOnly={!can(me, 'settings.update')} />
    );
}
