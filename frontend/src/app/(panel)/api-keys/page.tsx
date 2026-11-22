import type { Metadata } from 'next';
import { Card, CardContent } from '@/components/ui/card';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { ApiKey, RolesAndPermissions } from '@/lib/types';
import { PageHeading } from '../panel-header';
import { ApiKeyTable, CreateKeyDialog } from './keys-client';

export const metadata: Metadata = { title: 'API keys' };

export default async function ApiKeysPage() {
    const me = await requirePermission('api_keys.manage');
    const keys = (await api<ApiKey[]>('/api-keys')).data;
    // Describe scopes using the permission catalogue when it is visible.
    let catalogue: RolesAndPermissions['permissions'] = [];
    if (me.permissions.includes('users.manage')) {
        catalogue = (await api<RolesAndPermissions>('/roles')).data.permissions;
    }
    const scopes = me.permissions.map((key) => ({
        key,
        description: catalogue.find((p) => p.key === key)?.description ?? key,
    }));

    return (
        <>
            <PageHeading
                title="API keys"
                description="Credentials for integrations. Use them as `Authorization: Bearer msk_live_…`."
                actions={<CreateKeyDialog scopes={scopes} />}
            />
            <Card>
                <CardContent className="p-0">
                    <ApiKeyTable keys={keys} />
                </CardContent>
            </Card>
        </>
    );
}
