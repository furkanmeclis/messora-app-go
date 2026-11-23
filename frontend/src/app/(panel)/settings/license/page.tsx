import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { Schemas } from '@/lib/types';
import { LicenseView } from './license-view';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('license.title') };
}

export default async function LicenseSettingsPage() {
    const me = await requirePermission('settings.read');
    const info = (await api<Schemas['LicenseInfo']>('/license')).data;
    return <LicenseView info={info} isOwner={me.user.role.key === 'owner'} />;
}
