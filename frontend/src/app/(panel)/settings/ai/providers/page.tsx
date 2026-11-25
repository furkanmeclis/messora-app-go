import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Schemas } from '@/lib/types';
import { ProvidersClient } from './providers-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('ai.providers.title') };
}

export default async function AIProvidersPage() {
    const me = await requirePermission('ai.read');
    const [providers, settings] = await Promise.all([
        api<Schemas['AIProvider'][]>('/ai/providers'),
        api<Schemas['AISettings']>('/ai/settings'),
    ]);
    return (
        <ProvidersClient
            providers={providers.data}
            fakeAvailable={settings.data.fake_provider_available}
            canManage={can(me, 'ai.manage')}
        />
    );
}
