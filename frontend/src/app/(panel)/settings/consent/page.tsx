import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Schemas } from '@/lib/types';
import { ConsentForm } from './consent-form';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('consent.settings.tab') };
}

export default async function ConsentSettingsPage() {
    const me = await requirePermission('settings.read');
    const settings = (
        await api<Schemas['ConsentSettings']>('/settings/consent')
    ).data;
    return (
        <div className="max-w-5xl">
            <ConsentForm
                settings={settings}
                readOnly={!can(me, 'settings.update')}
            />
        </div>
    );
}
