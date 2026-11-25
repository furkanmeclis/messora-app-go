import type { Metadata } from 'next';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { Schemas } from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { EndpointDialog, EndpointTable } from './webhooks-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('webhooks.title') };
}

export default async function WebhooksPage() {
    await requirePermission('webhooks.manage');
    const [endpoints, catalogue, t] = await Promise.all([
        api<Schemas['WebhookEndpoint'][]>('/webhook-endpoints'),
        api<Schemas['WebhookEventTypeInfo'][]>('/webhook-event-types'),
        getT(),
    ]);
    return (
        <>
            <PageHeading
                title={t('webhooks.title')}
                description={t('webhooks.description')}
                actions={<EndpointDialog catalogue={catalogue.data} />}
            />
            <EndpointTable
                endpoints={endpoints.data}
                catalogue={catalogue.data}
            />
        </>
    );
}
