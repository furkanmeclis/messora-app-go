import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Schemas } from '@/lib/types';
import { ModelsClient } from './models-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('ai.sub.models') };
}

export default async function AIModelsPage() {
    const me = await requirePermission('ai.read');
    const [models, prices, settings] = await Promise.all([
        api<Schemas['AIModel'][]>('/ai/models'),
        api<Schemas['AIPrice'][]>('/ai/prices'),
        api<Schemas['AISettings']>('/ai/settings'),
    ]);
    return (
        <ModelsClient
            models={models.data}
            prices={prices.data}
            fakeAvailable={settings.data.fake_provider_available}
            canManage={can(me, 'ai.manage')}
        />
    );
}
