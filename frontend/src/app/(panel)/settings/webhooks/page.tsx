import type { Metadata } from 'next';
import { Card, CardContent } from '@/components/ui/card';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { Schemas } from '@/lib/types';
import { PageHeading } from '../../panel-header';
import { EndpointDialog, EndpointTable } from './webhooks-client';

export const metadata: Metadata = { title: 'Webhooks' };

export default async function WebhooksPage() {
    await requirePermission('webhooks.manage');
    const [endpoints, catalogue] = await Promise.all([
        api<Schemas['WebhookEndpoint'][]>('/webhook-endpoints'),
        api<Schemas['WebhookEventTypeInfo'][]>('/webhook-event-types'),
    ]);
    return (
        <>
            <PageHeading
                title="Webhooks"
                description="Messora sends signed HTTPS requests to these URLs when events happen. Verify the Messora-Signature header with the endpoint's secret."
                actions={<EndpointDialog catalogue={catalogue.data} />}
            />
            <Card>
                <CardContent className="p-0">
                    <EndpointTable
                        endpoints={endpoints.data}
                        catalogue={catalogue.data}
                    />
                </CardContent>
            </Card>
        </>
    );
}
