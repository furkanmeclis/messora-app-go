import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Schemas } from '@/lib/types';
import { ProtectionForm } from './protection-form';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('analytics.settings.title') };
}

export default async function CampaignProtectionPage() {
    const me = await requirePermission('settings.read');
    const settings = (
        await api<Schemas['CampaignProtectionSettings']>(
            '/settings/campaign-protection',
        )
    ).data;
    return (
        <div className="max-w-3xl">
            <ProtectionForm
                settings={settings}
                readOnly={!can(me, 'settings.update')}
            />
        </div>
    );
}
