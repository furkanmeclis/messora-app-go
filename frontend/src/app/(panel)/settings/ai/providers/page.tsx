import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import { hasFeature } from '@/lib/plan';
import type { Schemas } from '@/lib/types';
import { ProvidersClient } from './providers-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('ai.providers.title') };
}

export default async function AIProvidersPage() {
    const me = await requirePermission('ai.read');
    const [providers, settings, platform] = await Promise.all([
        api<Schemas['AIProvider'][]>('/ai/providers'),
        api<Schemas['AISettings']>('/ai/settings'),
        api<Schemas['AIPlatformProvider'][]>('/ai/platform-providers')
            .then((r) => r.data)
            .catch(() => [] as Schemas['AIPlatformProvider'][]),
    ]);
    return (
        <ProvidersClient
            providers={providers.data}
            fakeAvailable={settings.data.fake_provider_available}
            canManage={can(me, 'ai.manage')}
            byo={hasFeature(me, 'byo_ai_keys')}
            platform={platform}
        />
    );
}
