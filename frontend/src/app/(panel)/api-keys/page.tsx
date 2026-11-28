import type { Metadata } from 'next';
import { headers } from 'next/headers';
import Link from 'next/link';
import { BookOpen } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import { requestOrigin } from '@/lib/server/tenant-host';
import type { ApiKey, RolesAndPermissions } from '@/lib/types';
import { hasFeature } from '@/lib/plan';
import { UpgradeHint } from '@/components/plan-hint';
import { PageHeading } from '../panel-header';
import { ApiBaseCard, ApiKeyTable, CreateKeyDialog } from './keys-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('apiKeys.title') };
}

export default async function ApiKeysPage() {
    const me = await requirePermission('api_keys.manage');
    const t = await getT();
    const keys = (await api<ApiKey[]>('/api-keys')).data;
    // Describe scopes using the permission catalogue when it is visible.
    let catalogue: RolesAndPermissions['permissions'] = [];
    if (me.permissions.includes('users.manage')) {
        catalogue = (await api<RolesAndPermissions>('/roles')).data.permissions;
    }
    // The workspace's own API address (there is no shared API host).
    const apiBase =
        me.tenant?.api_base_url ?? `${requestOrigin(await headers())}/api/v1`;
    const scopes = me.permissions.map((key) => ({
        key,
        description: catalogue.find((p) => p.key === key)?.description ?? key,
    }));

    const publicApi = hasFeature(me, 'public_api');
    return (
        <>
            <PageHeading
                title={t('apiKeys.title')}
                description={t('apiKeys.description')}
                actions={
                    <>
                        <Button variant="outline" asChild>
                            <Link href="/docs/api" target="_blank">
                                <BookOpen />
                                {t('apiDocs.link')}
                            </Link>
                        </Button>
                        {publicApi && <CreateKeyDialog scopes={scopes} />}
                    </>
                }
            />
            {!publicApi && (
                <div className="mb-4">
                    <UpgradeHint testId="public-api-locked">
                        {t('plans.gate.apiKeys')}
                    </UpgradeHint>
                </div>
            )}
            <ApiBaseCard baseUrl={apiBase} />
            <ApiKeyTable keys={keys} />
        </>
    );
}
