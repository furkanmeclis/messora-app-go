import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import { hasFeature } from '@/lib/plan';
import type { Schemas } from '@/lib/types';
import { AISettingsForm } from './settings-form';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('ai.title') };
}

export default async function AISettingsPage() {
    const me = await requirePermission('ai.read');
    const [settings, providers, models, platform] = await Promise.all([
        api<Schemas['AISettings']>('/ai/settings'),
        api<Schemas['AIProvider'][]>('/ai/providers'),
        api<Schemas['AIModel'][]>('/ai/models'),
        api<Schemas['AIPlatformProvider'][]>('/ai/platform-providers')
            .then((r) => r.data)
            .catch(() => [] as Schemas['AIPlatformProvider'][]),
    ]);
    // Own keys are not used on plans without byo_ai_keys.
    const own = hasFeature(me, 'byo_ai_keys') ? providers.data : [];
    return (
        <div className="max-w-4xl">
            <AISettingsForm
                settings={settings.data}
                providers={own}
                platform={platform}
                models={models.data}
                readOnly={!can(me, 'ai.manage')}
            />
        </div>
    );
}
