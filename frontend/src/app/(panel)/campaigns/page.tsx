import type { Metadata } from 'next';
import Link from 'next/link';
import { Plus } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { api } from '@/lib/server/api';
import { can, requirePermission } from '@/lib/server/auth';
import type { Campaign } from '@/lib/types';
import { getT } from '@/i18n/server';
import { PageHeading } from '../panel-header';
import { CAMPAIGN_STATUSES } from './campaign-ui';
import { CampaignList, StatusFilter } from './campaigns-client';

export async function generateMetadata(): Promise<Metadata> {
    const t = await getT();
    return { title: t('campaigns.title') };
}

export default async function CampaignsPage({
    searchParams,
}: {
    searchParams: Promise<Record<string, string | undefined>>;
}) {
    const me = await requirePermission('campaigns.read');
    const sp = await searchParams;
    const status = CAMPAIGN_STATUSES.find((s) => s === sp.status) ?? '';
    const q = new URLSearchParams({ limit: '25' });
    if (status) q.set('status', status);
    const [list, t] = await Promise.all([
        api<Campaign[]>(`/campaigns?${q}`),
        getT(),
    ]);

    return (
        <>
            <PageHeading
                title={t('campaigns.title')}
                description={t('campaigns.description')}
                actions={
                    can(me, 'campaigns.create') ? (
                        <Button asChild shortcut="n">
                            <Link href="/campaigns/new">
                                <Plus /> {t('campaigns.newCampaign')}
                            </Link>
                        </Button>
                    ) : undefined
                }
            />
            <div className="space-y-4">
                <StatusFilter status={status} />
                <Card>
                    <CardContent className="p-0">
                        <CampaignList
                            key={status}
                            initial={list.data}
                            initialCursor={list.nextCursor}
                            status={status}
                            canCreate={can(me, 'campaigns.create')}
                        />
                    </CardContent>
                </Card>
            </div>
        </>
    );
}
