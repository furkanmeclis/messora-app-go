import type { Metadata } from 'next';
import Link from 'next/link';
import { Plus } from 'lucide-react';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { getT } from '@/i18n/server';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Schemas } from '@/lib/types';
import { PageHeading } from '../panel-header';
import { ProfilesTable } from './profiles-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('ai.profiles.title') };
}

export default async function AIProfilesPage() {
    const me = await requirePermission('ai.read');
    const [profiles, providers, settings, t] = await Promise.all([
        api<Schemas['AIProfile'][]>('/ai/profiles'),
        api<Schemas['AIProvider'][]>('/ai/providers'),
        api<Schemas['AISettings']>('/ai/settings'),
        getT(),
    ]);
    const canManage = can(me, 'ai.manage');
    return (
        <>
            <PageHeading
                title={t('ai.profiles.title')}
                description={t('ai.profiles.description')}
                actions={
                    canManage && (
                        <Button asChild>
                            <Link href="/ai-profiles/new">
                                <Plus /> {t('ai.profiles.new')}
                            </Link>
                        </Button>
                    )
                }
            />
            {!settings.data.enabled && (
                <Alert className="mb-4">
                    <AlertDescription>
                        {t('ai.profiles.aiOff')}
                    </AlertDescription>
                </Alert>
            )}
            <ProfilesTable
                profiles={profiles.data}
                providers={providers.data}
                settings={settings.data}
                canManage={canManage}
            />
        </>
    );
}
