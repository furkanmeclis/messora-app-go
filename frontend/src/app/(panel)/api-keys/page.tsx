import type { Metadata } from 'next';
import Link from 'next/link';
import { BookOpen } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { requirePermission } from '@/lib/server/auth';
import type { ApiKey, RolesAndPermissions } from '@/lib/types';
import { PageHeading } from '../panel-header';
import { ApiKeyTable, CreateKeyDialog } from './keys-client';

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
    const scopes = me.permissions.map((key) => ({
        key,
        description: catalogue.find((p) => p.key === key)?.description ?? key,
    }));

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
                        <CreateKeyDialog scopes={scopes} />
                    </>
                }
            />
            <ApiKeyTable keys={keys} />
        </>
    );
}
